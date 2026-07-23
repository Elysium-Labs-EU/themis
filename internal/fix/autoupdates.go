package fix

import (
	"encoding/json"
	"fmt"
)

const autoUpgradesConfigPath = "/etc/apt/apt.conf.d/20auto-upgrades"

type autoUpdatesState struct {
	PrevConfig    []byte `json:"prev_config"`
	WasInstalled  bool   `json:"was_installed"`
	ConfigExisted bool   `json:"config_existed"`
}

func autoUpdatesFix() Fix {
	return autoUpdatesFixWith(autoUpgradesConfigPath, runCmd, packageInstalled)
}

// autoUpdatesFixWith builds the PKGS-7392 fix with the config path and
// effect seams parameterized, so Check/Apply/Revert are unit-testable
// against a temp file with fake runners.
func autoUpdatesFixWith(path string, run cmdRunner, pkgInstalled pkgChecker) Fix {
	return Fix{
		TestID:      "PKGS-7392",
		Description: "enable unattended-upgrades for automatic security updates",
		Check:       func() (bool, error) { return autoUpdatesCheck(path, pkgInstalled) },
		Apply:       func() ([]byte, error) { return autoUpdatesApply(path, run, pkgInstalled) },
		Revert:      func(data []byte) error { return autoUpdatesRevert(data, path, run) },
		RevertWarn:  func(data []byte) (string, bool, error) { return autoUpdatesRevertWarn(data, path) },
	}
}

// autoUpdatesApplied renders the config content Apply writes given
// prevConfig, the pre-apply content. Pure — no I/O — shared by Apply and
// RevertWarn so drift detection compares against exactly what Apply wrote.
func autoUpdatesApplied(prevConfig []byte) string {
	updated := setDirective(string(prevConfig), "APT::Periodic::Unattended-Upgrade", `"1";`)
	return setDirective(updated, "APT::Periodic::Update-Package-Lists", `"1";`)
}

// autoUpdatesCheck reports whether unattended-upgrades is installed and the
// config at path enables the Unattended-Upgrade periodic directive.
func autoUpdatesCheck(path string, pkgInstalled pkgChecker) (bool, error) {
	if !pkgInstalled("unattended-upgrades") {
		return false, nil
	}
	content, existed, err := ReadFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	if !existed {
		return false, nil
	}
	return DirectiveValue(string(content), "APT::Periodic::Unattended-Upgrade") == `"1";`, nil
}

// autoUpdatesApply installs unattended-upgrades if needed, sets the periodic
// directives in the config at path, and returns the JSON revert state.
func autoUpdatesApply(path string, run cmdRunner, pkgInstalled pkgChecker) ([]byte, error) {
	wasInstalled := pkgInstalled("unattended-upgrades")
	if !wasInstalled {
		if err := run("apt-get", "install", "-y", "unattended-upgrades"); err != nil {
			return nil, err
		}
	}
	original, existed, err := ReadFileOrEmpty(path)
	if err != nil {
		return nil, err
	}
	updated := autoUpdatesApplied(original)
	if writeErr := writeFile(path, []byte(updated), 0o644); writeErr != nil {
		return nil, writeErr
	}
	state := autoUpdatesState{WasInstalled: wasInstalled, PrevConfig: original, ConfigExisted: existed}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling auto-updates revert state: %w", err)
	}
	return data, nil
}

// autoUpdatesRevert restores the config at path (or removes it if it didn't
// exist) and undoes the install using the JSON revert state.
func autoUpdatesRevert(data []byte, path string, run cmdRunner) error {
	var state autoUpdatesState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshaling auto-updates revert state: %w", err)
	}
	if state.ConfigExisted {
		if err := writeFile(path, state.PrevConfig, 0o644); err != nil {
			return err
		}
	} else if err := removeFile(path); err != nil {
		return err
	}
	if !state.WasInstalled {
		return run("apt-get", "remove", "-y", "unattended-upgrades")
	}
	return nil
}

// autoUpdatesRevertWarn reports whether path currently differs from the
// content Apply wrote, i.e. it was hand-edited since apply and Revert
// would discard that edit unless warned first.
func autoUpdatesRevertWarn(data []byte, path string) (string, bool, error) {
	var state autoUpdatesState
	if err := json.Unmarshal(data, &state); err != nil {
		return "", false, fmt.Errorf("unmarshaling auto-updates revert state: %w", err)
	}
	return revertDrifted(path, autoUpdatesApplied(state.PrevConfig))
}
