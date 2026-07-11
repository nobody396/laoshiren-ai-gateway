export type ApiDomainEvent =
  | { type: 'session-expired' }
  | { type: 'forbidden'; messageKey: 'common.permissionDenied' }
  | { type: 'ops-monitoring-disabled' }

let eventHandler: ((event: ApiDomainEvent) => void) | null = null

export function configureApiRuntime(handler: (event: ApiDomainEvent) => void): void {
  eventHandler = handler
}

export function emitApiEvent(event: ApiDomainEvent): void {
  eventHandler?.(event)
}
