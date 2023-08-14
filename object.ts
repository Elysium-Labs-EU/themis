import { TypeGuard } from "./dataValidators";

// export interface ObjectSchema<TIn extends Maybe<AnyObjecty>, TContext = AnyObject>>{}

export const object: TypeGuard<Record<string, any>> = (value: unknown, path: string[]) => {
  if (typeof value !== 'object' || value === null) {
    throw new Error(`Key '${path.join('.')}' value is not an object`);
  }
  return value
}

export const objectOf = <T extends Record<string, TypeGuard<any>>>(
  inner: T
) => {
  return (value: unknown, path: string[]): { [K in keyof T]: ReturnType<T[K]> } => {
    const valueAsObject = object(value, path)
    const out: { [P in keyof T]: ReturnType<T[P]> } = {} as any
    for (const key in inner) {
      const innerTypeGuard = inner[key]
      if (innerTypeGuard) {
        out[key] = innerTypeGuard((valueAsObject as any)[key], [...path, key])
      }
    }
    return out
  }
}
