// Calls into the Go side. Wails exposes the bound methods of gui.App as
// window.go.gui.App.<Method>; each returns a promise, rejected with the
// already-localized error text.

export interface FileEntry {
  path: string; origPath?: string; index: string; worktree: string
  staged: boolean; unstaged: boolean; untracked: boolean; conflicted: boolean
}
export interface BranchStatus {
  Head: string; Detached: boolean; Unborn: boolean; OID: string
  Upstream: string; UpstreamGone: boolean; Ahead: number; Behind: number
}
export interface RepoState { root: string; branch: BranchStatus; operation: string; files: FileEntry[]; protected: boolean }
export interface DiffLine { Kind: number; Text: string; OldLine: number; NewLine: number }
export interface Hunk { OldStart: number; OldLines: number; NewStart: number; NewLines: number; Section: string; Lines: DiffLine[] }
export interface FileDiff { Path: string; OldPath: string; Change: string; Binary: boolean; Hunks: Hunk[] }
export interface Branch { Name: string; Ref: string; Remote: boolean; Current: boolean; Upstream: string; Ahead: number; Behind: number; Subject: string }
export interface Commit { Hash: string; ShortHash: string; Parents: string[]; AuthorName: string; AuthorDate: string; Subject: string; Body: string; Refs: string[] | null }
export interface LogEntry { commit: Commit; graph: string }
export interface Assessment {
  risk: 'safe' | 'caution' | 'dangerous' | 'critical'; riskLabel: string
  confirm: 'none' | 'simple' | 'explicit' | 'type-branch'; confirmText: string
  blocked: boolean; effects: string[]; findings: string[]; backups: string[]
}
export interface OpRequest {
  kind: string; message?: string; amend?: boolean; paths?: string[]; branch?: string
  rev?: string; mode?: string; force?: boolean; strategy?: string; stashIndex?: number
}
export interface Outcome { message: string; backup: string }
export interface Info { version: string; lang: string; theme: string; texts: Record<string, string>; root: string }
export interface HistoryEntry { id: string; time: string; op: string; branch?: string; before?: string; after?: string; failed?: string }
export interface Backup { Ref: string; Hash: string; Time: string; Op: string; Worktree: boolean }
export interface Command { line: string; exitCode: number; millis: number }
export interface UndoInfo { possible: boolean; op: string; when: string; reason: string }
export interface AIStatus { enabled: boolean; consent: boolean; hasKey: boolean; name: string; notice: string }
export interface Suggestion { messages: string[]; split: string; sent: string; withheld: string[] }
export interface Stash { Index: number; Ref: string; Message: string }
export interface Tag { Name: string; Commit: string }
export interface Remote { Name: string; FetchURL: string }
export interface Identity { Name: string; Email: string }

type Bound = Record<string, (...args: unknown[]) => Promise<unknown>>

function app(): Bound {
  const go = (window as unknown as { go?: { gui?: { App?: Bound } } }).go
  if (!go?.gui?.App) throw new Error('gitpal backend is not available')
  return go.gui.App
}

function call<T>(name: string, ...args: unknown[]): Promise<T> {
  return app()[name](...args) as Promise<T>
}

export const api = {
  info: () => call<Info>('Info'),
  recentRepos: () => call<string[]>('RecentRepos'),
  openRepo: (dir: string) => call<string>('OpenRepo', dir),
  initRepo: (dir: string) => call<string>('InitRepo', dir),
  cloneRepo: (url: string, dir: string) => call<string>('CloneRepo', url, dir),
  state: () => call<RepoState>('State'),
  diff: (path: string, staged: boolean) => call<FileDiff[]>('Diff', path, staged),
  stage: (paths: string[]) => call<void>('Stage', paths),
  stageAll: () => call<void>('StageAll'),
  unstage: (paths: string[]) => call<void>('Unstage', paths),
  resolve: (paths: string[], side: '' | 'ours' | 'theirs') => call<void>('Resolve', paths, side),
  pickFolder: () => call<string>('PickFolder'),
  stageHunks: (path: string, staged: boolean, hunks: number[]) => call<void>('StageHunks', path, staged, hunks),
  branches: () => call<Branch[]>('Branches'),
  stashes: () => call<Stash[]>('Stashes'),
  tags: () => call<Tag[]>('Tags'),
  remotes: () => call<Remote[] | null>('Remotes'),
  log: (max: number, all: boolean, search: string) => call<LogEntry[]>('Log', max, all, search),
  commitFiles: (rev: string) => call<FileDiff[] | null>('CommitFiles', rev),
  assess: (op: OpRequest) => call<Assessment>('Assess', op),
  run: (op: OpRequest, answer: { approved: boolean; typed: string }) => call<Outcome>('Do', op, answer),
  fetch: () => call<void>('Fetch'),
  createBranch: (name: string) => call<void>('CreateBranch', name),
  historyEntries: () => call<HistoryEntry[]>('HistoryEntries'),
  backups: () => call<Backup[]>('Backups'),
  planUndo: () => call<UndoInfo>('PlanUndo'),
  undo: () => call<void>('Undo'),
  commands: () => call<Command[]>('Commands'),
  settings: () => call<Record<string, string>>('Settings'),
  setSetting: (key: string, value: string) => call<void>('SetSetting', key, value),
  identity: () => call<Identity>('Identity'),
  setIdentity: (name: string, email: string, global: boolean) => call<void>('SetIdentity', name, email, global),
  setAIKey: (key: string) => call<void>('SetAIKey', key),
  aiStatus: () => call<AIStatus>('AIStatus'),
  giveAIConsent: () => call<void>('GiveAIConsent'),
  suggestCommit: () => call<Suggestion>('SuggestCommit'),
}

// Line kinds as git marks them (git.LineKind is a byte).
export const LINE = { context: 32, added: 43, deleted: 45, noNewline: 92 }
