export interface Contact {
  peer_ref: string
  display_name: string
  username?: string
  phone?: string
  avatar_initial?: string
  has_dialog: boolean
}

export interface ContactsResponse {
  ok: boolean
  contacts: Contact[]
  source?: string
  stale?: boolean
  error?: string
}
