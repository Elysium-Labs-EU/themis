export type Maybe<T> = T | undefined | null

export type AnyObject = Record<string, any>

export type Optionals<T> = Extract<T, undefined | null>

export type Defined<T> = T extends undefined ? never : T

export type NonNull<T> = T extends null ? never : T
