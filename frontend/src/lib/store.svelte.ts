// Shared state of the app: texts, the open repository, the safety dialog and
// notifications.
import { api, type Assessment, type OpRequest, type Outcome, type RepoState } from './api'

export const app = $state({
  texts: {} as Record<string, string>,
  version: '',
  theme: 'system',
  root: '',
  state: null as RepoState | null,
  toast: '' as string,
  error: '' as string,
  busy: false,
})

// t returns a text from the Go catalog; the key itself when missing.
export function t(key: string): string {
  return app.texts['gui.' + key] ?? key
}

export function applyTheme(theme: string) {
  const dark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

export async function loadInfo() {
  const info = await api.info()
  app.texts = info.texts
  app.version = info.version
  app.theme = info.theme
  document.documentElement.lang = info.lang
  applyTheme(info.theme)
  if (info.root && !app.root) {
    app.root = info.root
    await refresh()
  }
}

export async function refresh() {
  if (!app.root) return
  try {
    app.state = await api.state()
  } catch (e) {
    fail(e)
  }
}

export function fail(e: unknown) {
  app.error = e instanceof Error ? e.message : String(e)
}

export function notify(message: string) {
  app.toast = message
  setTimeout(() => { if (app.toast === message) app.toast = '' }, 4000)
}

// The safety dialog: what is being asked, and how it ends.
export const dialog = $state({
  open: false,
  op: null as OpRequest | null,
  assessment: null as Assessment | null,
  resolve: null as ((o: Outcome | null) => void) | null,
})

// guarded runs an operation the way the safety layer wants: assess, show the
// dialog when a confirmation (or a warning) is due, then run it. Resolves
// with null when the user cancels.
export async function guarded(op: OpRequest): Promise<Outcome | null> {
  app.error = ''
  let assessment: Assessment
  try {
    assessment = await api.assess(op)
  } catch (e) {
    fail(e)
    return null
  }
  if (assessment.confirm === 'none' && !assessment.blocked && assessment.findings.length === 0) {
    return execute(op, true, '')
  }
  return new Promise((resolve) => {
    dialog.op = op
    dialog.assessment = assessment
    dialog.resolve = resolve
    dialog.open = true
  })
}

export async function execute(op: OpRequest, approved: boolean, typed: string): Promise<Outcome | null> {
  app.busy = true
  try {
    const out = await api.run(op, { approved, typed })
    notify(out.message + (out.backup ? ` · ${out.backup}` : ''))
    return out
  } catch (e) {
    fail(e)
    return null
  } finally {
    app.busy = false
    await refresh()
  }
}
