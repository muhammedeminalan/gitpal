<script lang="ts">
  import { api, type FileDiff, type LogEntry } from '../lib/api'
  import { app, t, fail, guarded } from '../lib/store.svelte'

  let entries = $state<LogEntry[]>([])
  let all = $state(false)
  let search = $state('')
  let selected = $state<LogEntry | null>(null)
  let files = $state<FileDiff[]>([])

  async function load() {
    try { entries = await api.log(300, all, search) } catch (e) { fail(e) }
  }
  $effect(() => { void all; void app.state?.branch?.OID; load() })

  async function pick(e: LogEntry) {
    selected = e
    try { files = (await api.commitFiles(e.commit.Hash)) ?? [] } catch (err) { fail(err); files = [] }
  }

  async function op(kind: string, extra: Record<string, string> = {}) {
    if (!selected) return
    await guarded({ kind, rev: selected.commit.Hash, ...extra })
    load()
  }
</script>

<section class="flex-1 min-w-0 flex flex-col">
  <div class="flex gap-2 p-2 border-b border-zinc-200 dark:border-zinc-700">
    <form class="flex-1" onsubmit={(e) => { e.preventDefault(); load() }}>
      <input class="input" placeholder={t('history.search')} bind:value={search} />
    </form>
    <label class="flex items-center gap-1 text-xs"><input type="checkbox" bind:checked={all} />{t('history.all')}</label>
  </div>
  <div class="flex-1 overflow-auto">
    {#each entries as e (e.commit.Hash)}
      <button class="w-full text-left flex gap-2 px-3 py-0.5 {selected?.commit.Hash === e.commit.Hash ? 'bg-blue-100 dark:bg-blue-900' : 'hover:bg-zinc-100 dark:hover:bg-zinc-800'}" onclick={() => pick(e)}>
        <span class="mono text-blue-600 whitespace-pre shrink-0">{e.graph}</span>
        <span class="mono text-zinc-500 shrink-0">{e.commit.ShortHash}</span>
        {#each e.commit.Refs ?? [] as r}<span class="mono text-xs px-1 rounded bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-100 shrink-0">{r}</span>{/each}
        <span class="truncate flex-1">{e.commit.Subject}</span>
        <span class="text-zinc-500 shrink-0 text-xs">{e.commit.AuthorName}</span>
      </button>
    {/each}
  </div>
</section>

{#if selected}
  <aside class="w-96 shrink-0 border-l border-zinc-200 dark:border-zinc-700 overflow-auto p-3 space-y-3">
    <div class="mono text-xs text-zinc-500">{selected.commit.Hash}</div>
    <div class="font-semibold">{selected.commit.Subject}</div>
    {#if selected.commit.Body}<pre class="whitespace-pre-wrap text-sm">{selected.commit.Body}</pre>{/if}
    <div class="text-xs text-zinc-500">{selected.commit.AuthorName} · {new Date(selected.commit.AuthorDate).toLocaleString()}</div>
    <div class="flex flex-wrap gap-2">
      <button class="btn" onclick={() => op('revert')}>{t('history.revert')}</button>
      <button class="btn" onclick={() => op('cherry-pick')}>{t('history.cherry')}</button>
      <button class="btn text-red-600" onclick={() => op('reset', { mode: 'mixed' })}>{t('history.reset')}</button>
    </div>
    <div class="panel-title px-0">{t('history.files')}</div>
    {#each files as f}<div class="mono text-xs truncate"><span class="text-zinc-500 w-16 inline-block">{f.Change}</span>{f.Path}</div>{/each}
  </aside>
{/if}
