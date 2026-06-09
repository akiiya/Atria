export interface ApiOk<T> {
  ok: true
  data?: T
}

export interface ApiErr {
  ok: false
  code: string
  message: string
}

export type ApiResult<T = void> = ApiOk<T> & T | ApiErr
