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
	return Fix{
		TestID:      "PKGS-7392",
		Description: "enable unattended-upgrades for automatic security updates",
		Check: func() (bool, error) {
			if !packageInstalled("unattended-upgrades") {
				return false, nil
			}
			content, existed, err := ReadFileOrEmpty(autoUpgradesConfigPath)
			if err != nil {
				return false, err
			}
			if !existed {
				return false, nil
			}
			return DirectiveValue(string(content), "APT::Periodic::Unattended-Upgrade") == `"1";`, nil
		},
		Apply: func() ([]byte, error) {
			wasInstalled := packageInstalled("unattended-upgrades")
			if !wasInstalled {
				if err := runCmd("apt-get", "install", "-y", "unattended-upgrades"); err != nil {
					return nil, err
				}
			}
			original, existed, err := ReadFileOrEmpty(autoUpgradesConfigPath)
			if err != nil {
				return nil, err
			}
			updated := setDirective(string(original), "APT::Periodic::Unattended-Upgrade", `"1";`)
			updated = setDirective(updated, "APT::Periodic::Update-Package-Lists", `"1";`)
			if writeErr := writeFile(autoUpgradesConfigPath, []byte(updated), 0o644); writeErr != nil {
				return nil, writeErr
			}
			state := autoUpdatesState{WasInstalled: wasInstalled, PrevConfig: original, ConfigExisted: existed}
			data, err := json.Marshal(state)
			if err != nil {
				return nil, fmt.Errorf("marshaling auto-updates revert state: %w", err)
			}
			return data, nil
		},
		Revert: func(data []byte) error {
			var state autoUpdatesState
			if err := json.Unmarshal(data, &state); err != nil {
				return fmt.Errorf("unmarshaling auto-updates revert state: %w", err)
			}
			if state.ConfigExisted {
				if err := writeFile(autoUpgradesConfigPath, state.PrevConfig, 0o644); err != nil {
					return err
				}
			} else if err := removeFile(autoUpgradesConfigPath); err != nil {
				return err
			}
			if !state.WasInstalled {
				return runCmd("apt-get", "remove", "-y", "unattended-upgrades")
			}
			return nil
		},
	}
}
