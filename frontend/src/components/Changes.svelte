<script lang="ts">
  import { api, LINE, type FileDiff, type FileEntry } from '../lib/api'
  import { app, t, fail, guarded, refresh } from '../lib/store.svelte'
  import CommitBox from './CommitBox.svelte'

  let selected = $state<{ path: string; staged: boolean } | null>(null)
  let diffs = $state<FileDiff[]>([])

  const files = $derived(app.state?.files ?? [])
  const staged = $derived(files.filter((f) => f.staged))
  const conflicted = $derived(files.filter((f) => f.conflicted))
  const unstaged = $derived(files.filter((f) => (f.unstaged || f.untracked) && !f.conflicted))

  async function show(path: string, isStaged: boolean) {
    selected = { path, staged: isStaged }
    try { diffs = await api.diff(path, isStaged) } catch (e) { fail(e); diffs = [] }
  }

  // Keep the shown diff in step with the file list; drop it when the file
  // has nothing left on that side.
  $effect(() => {
    const s = selected
    if (!s) return
    const still = (s.staged ? staged : [...unstaged, ...conflicted]).some((f) => f.path === s.path)
    if (!still) { selected = null; diffs = [] } else api.diff(s.path, s.staged).then((d) => (diffs = d)).catch(() => {})
  })

  async function act(p: Promise<unknown>) {
    try { await p } catch (e) { fail(e) }
    await refresh()
  }

  function badge(f: FileEntry, side: 'index' | 'worktree') {
    if (f.conflicted) return 'U'
    if (f.untracked) return '?'
    return f[side] === '.' ? '' : f[side]
  }

  function onKey(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement).tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || !selected) return
    if (e.key === 's') { e.preventDefault(); act(selected.staged ? api.unstage([selected.path]) : api.stage([selected.path])) }
  }

  const lineClass = (k: number) =>
    k === LINE.added ? 'bg-emerald-50 dark:bg-emerald-950' : k === LINE.deleted ? 'bg-red-50 dark:bg-red-950' : k === LINE.noNewline ? 'text-zinc-400' : ''
</script>

<svelte:window onkeydown={onKey} />

<section class="w-80 shrink-0 border-r border-zinc-200 dark:border-zinc-700 flex flex-col min-h-0">
  <div class="flex-1 overflow-auto py-2">
    {#if conflicted.length}
      <div class="panel-title text-red-600">{t('conflict.title')} ({conflicted.length})</div>
      {#each conflicted as f (f.path)}
        <div class="px-3 py-1 space-y-1 {selected?.path === f.path ? 'bg-red-50 dark:bg-red-950' : ''}">
          <button class="mono truncate block w-full text-left text-red-700 dark:text-red-300" title={f.path} onclick={() => show(f.path, false)}>U {f.path}</button>
          <div class="flex flex-wrap gap-1 text-xs">
            <button class="btn py-0.5" onclick={() => act(api.resolve([f.path], 'ours'))}>{t('conflict.ours')}</button>
            <button class="btn py-0.5" onclick={() => act(api.resolve([f.path], 'theirs'))}>{t('conflict.theirs')}</button>
            <button class="btn py-0.5" onclick={() => act(api.resolve([f.path], ''))}>{t('conflict.resolved')}</button>
          </div>
        </div>
      {/each}
    {/if}
    <div class="panel-title flex items-center">
      <span class="flex-1">{t('files.unstaged')} ({unstaged.length})</span>
      {#if unstaged.length}<button class="normal-case text-blue-600" onclick={() => act(api.stageAll())}>{t('files.stage_all')}</button>{/if}
    </div>
    {#each unstaged as f (f.path)}
      <div class="group flex items-center gap-2 px-3 py-0.5 cursor-pointer {selected?.path === f.path && !selected.staged ? 'bg-blue-100 dark:bg-blue-900' : 'hover:bg-zinc-100 dark:hover:bg-zinc-800'}"
        role="button" tabindex="0" onclick={() => show(f.path, false)} onkeydown={(e) => e.key === 'Enter' && show(f.path, false)}>
        <span class="mono w-4 text-amber-600">{badge(f, 'worktree')}</span>
        <span class="mono truncate flex-1" title={f.path}>{f.path}</span>
        <span class="hidden group-hover:flex gap-1 text-xs">
          {#if !f.untracked}<button class="text-red-600" title={t('files.discard')} onclick={(e) => { e.stopPropagation(); guarded({ kind: 'discard', paths: [f.path] }) }}>↺</button>{/if}
          <button title={t('files.stage')} onclick={(e) => { e.stopPropagation(); act(api.stage([f.path])) }}>＋</button>
        </span>
      </div>
    {/each}
    <div class="panel-title mt-3">{t('files.staged')} ({staged.length})</div>
    {#each staged as f (f.path)}
      <div class="group flex items-center gap-2 px-3 py-0.5 cursor-pointer {selected?.path === f.path && selected.staged ? 'bg-blue-100 dark:bg-blue-900' : 'hover:bg-zinc-100 dark:hover:bg-zinc-800'}"
        role="button" tabindex="0" onclick={() => show(f.path, true)} onkeydown={(e) => e.key === 'Enter' && show(f.path, true)}>
        <span class="mono w-4 text-emerald-600">{badge(f, 'index')}</span>
        <span class="mono truncate flex-1" title={f.path}>{f.path}</span>
        <button class="hidden group-hover:inline text-xs" title={t('files.unstage')} onclick={(e) => { e.stopPropagation(); act(api.unstage([f.path])) }}>－</button>
      </div>
    {/each}
    {#if files.length === 0}<p class="px-3 py-4 text-zinc-500">{t('files.empty')}</p>{/if}
  </div>
  <CommitBox />
</section>

<section class="flex-1 min-w-0 overflow-auto">
  {#if !selected}
    <p class="p-6 text-zinc-500">{t('diff.select')}</p>
  {:else}
    {#each diffs as d}
      <div class="sticky top-0 bg-white dark:bg-zinc-900 border-b border-zinc-200 dark:border-zinc-700 px-3 py-1.5 mono font-semibold">
        {d.OldPath && d.OldPath !== d.Path ? `${d.OldPath} → ` : ''}{d.Path}
      </div>
      {#if d.Binary}<p class="p-3 text-zinc-500">{t('diff.binary')}</p>{/if}
      {#each d.Hunks as h, i}
        <div class="flex items-center bg-zinc-50 dark:bg-zinc-800 text-zinc-500 mono text-xs px-3 py-1">
          <span class="flex-1">@@ -{h.OldStart},{h.OldLines} +{h.NewStart},{h.NewLines} @@ {h.Section}</span>
          <button class="btn text-xs py-0.5" onclick={() => act(api.stageHunks(selected!.path, selected!.staged, [i]))}>
            {selected.staged ? t('diff.unstage_hunk') : t('diff.stage_hunk')}
          </button>
        </div>
        <pre class="mono text-xs leading-5">{#each h.Lines as l}<div class="flex {lineClass(l.Kind)}"><span class="w-10 shrink-0 text-right pr-2 text-zinc-400 select-none">{l.OldLine || ''}</span><span class="w-10 shrink-0 text-right pr-2 text-zinc-400 select-none">{l.NewLine || ''}</span><span class="whitespace-pre">{String.fromCharCode(l.Kind)}{l.Text}</span></div>{/each}</pre>
      {/each}
    {/each}
  {/if}
</section>
