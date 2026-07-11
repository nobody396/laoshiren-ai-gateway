export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code?: string | number,
    message = 'Unknown error',
    public readonly details: { reason?: unknown; error?: unknown; url?: string } = {},
  ) {
    super(message)
    this.name = 'ApiError'
  }

  get reason(): unknown { return this.details.reason }
  get error(): unknown { return this.details.error }
  get url(): string | undefined { return this.details.url }
}
