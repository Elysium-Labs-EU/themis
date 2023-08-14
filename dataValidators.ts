export type TypeGuard<T> = (value: unknown, path: string[]) => T

export const union = <T extends TypeGuard<any>[]>(...guards: T) => {
  return (value: unknown, path: string[]): ReturnType<T[number]> => {
    const out: any = {}
    let index = 0
    for (const guard of guards) {

      out[index = + 1] = guard(value, path)
    }
    return out
  }
}

export const optional = <T extends TypeGuard<any>>(guard: T) => {
  return (value: unknown, path: string[]): ReturnType<T> | undefined => {
    if (typeof value === 'undefined') {
      return undefined
    }
    return guard(value, path)
  }
}

export const string: TypeGuard<string> = (value: unknown, path: string[]) => {
  if (typeof value !== 'string') {
    console.log('path', path)
    throw new Error(`Key '${path.join('.')}' is not a string`)
  }
  return value
}

export const number: TypeGuard<number> = (value: unknown, path: string[]) => {
  if (typeof value !== 'number') {
    throw new Error(`Key '${path.join('.')} is not a number`)
  }
  return value
}

export const isUndefined: TypeGuard<undefined> = (value: unknown, path: string[]) => {
  if (typeof value !== 'undefined') {
    throw new Error(`Key '${path.join('.')} is not undefined`)
  }
  return value
}

export const isNull: TypeGuard<null> = (value: unknown, path: string[]) => {
  if (value !== null) {
    throw new Error(`Key '${path.join('.')} is not null`)
  }
  return value
}

export const isAny: TypeGuard<any> = (value: unknown) => value

export const boolean: TypeGuard<boolean> = (value: unknown, path: string[]) => {
  if (typeof value !== 'boolean') {
    throw new Error(`Key '${path.join('.')} is not a boolean`)
  }
  return value
}

export const array =
  <T>(inner: TypeGuard<T>) =>
    (value: unknown, path: string[]): T[] => {
      if (!Array.isArray(value)) {
        throw new Error(`Key '${path.join('.')} is not an array`)
      }
      return value.map((item, index) => inner(item, [...path, String(index)]))
    }


export const enumType = <T extends string, U extends [T, ...T[]]>(
  values: U
) => {
  return (value: unknown, path: string[]): U[number] => {
    if (!values.includes(value as any)) {
      throw new Error(`Key '${path.join('.')}' value is not a valid enum value`)
    }
    return value as U[number]
  }
}
