<script lang="ts">
  import { api, type Branch, type Remote, type Stash, type Tag } from '../lib/api'
  import { app, t, fail, guarded, refresh, notify } from '../lib/store.svelte'

  let branches = $state<Branch[]>([])
  let stashes = $state<Stash[]>([])
  let tags = $state<Tag[]>([])
  let remotes = $state<Remote[]>([])
  let newBranch = $state('')

  async function load() {
    try {
      ;[branches, stashes, tags, remotes] = await Promise.all([
        api.branches(), api.stashes(), api.tags(), api.remotes().then((r) => r ?? []),
      ])
    } catch (e) { fail(e) }
  }

  // Reload whenever the head or the operation in progress changes.
  $effect(() => { void app.state?.branch?.Head; void app.state?.branch?.OID; load() })

  async function create() {
    try { await api.createBranch(newBranch); notify(newBranch + ' ✓'); newBranch = ''; await refresh(); load() } catch (e) { fail(e) }
  }
  const after = (p: Promise<unknown>) => p.then(load)
</script>

<aside class="w-64 shrink-0 border-r border-zinc-200 dark:border-zinc-700 overflow-auto py-2">
  <div class="panel-title">{t('side.branches')}</div>
  {#each branches.filter((b) => !b.Remote) as b (b.Ref)}
    <div class="group flex items-center gap-1 px-3 py-1 hover:bg-zinc-100 dark:hover:bg-zinc-800">
      <span class="mono truncate flex-1 {b.Current ? 'font-semibold text-blue-600' : ''}" title={b.Subject}>{b.Name}</span>
      {#if !b.Current}
        <span class="hidden group-hover:flex gap-1 text-xs">
          <button title={t('side.switch')} onclick={() => after(guarded({ kind: 'switch', branch: b.Name }))}>⇄</button>
          <button title={t('side.merge')} onclick={() => after(guarded({ kind: 'merge', rev: b.Name }))}>⤵</button>
          <button title={t('side.delete')} class="text-red-600" onclick={() => after(guarded({ kind: 'delete-branch', branch: b.Name }))}>✕</button>
        </span>
      {/if}
    </div>
  {/each}
  <form class="flex gap-1 px-3 py-1" onsubmit={(e) => { e.preventDefault(); if (newBranch) create() }}>
    <input class="input mono text-xs" placeholder={t('side.new_branch')} bind:value={newBranch} />
  </form>

  <div class="panel-title mt-3">{t('side.remotes')}</div>
  {#each remotes as r}<div class="px-3 py-0.5 mono truncate" title={r.FetchURL}>{r.Name}</div>{/each}
  {#each branches.filter((b) => b.Remote) as b (b.Ref)}
    <button class="block w-full text-left px-5 py-0.5 mono text-xs text-zinc-500 truncate hover:bg-zinc-100 dark:hover:bg-zinc-800"
      title={t('side.switch')} onclick={() => after(guarded({ kind: 'switch', branch: b.Name.replace(/^[^/]+\//, '') }))}>{b.Name}</button>
  {/each}

  <div class="panel-title mt-3">{t('side.stashes')}</div>
  {#each stashes as s (s.Ref)}
    <div class="group flex gap-1 px-3 py-0.5">
      <span class="truncate flex-1" title={s.Message}>{s.Ref}: {s.Message}</span>
      <button class="hidden group-hover:inline text-xs text-red-600" title={t('side.delete')} onclick={() => after(guarded({ kind: 'stash-drop', stashIndex: s.Index }))}>✕</button>
    </div>
  {/each}

  <div class="panel-title mt-3">{t('side.tags')}</div>
  {#each tags as tg}<div class="px-3 py-0.5 mono truncate">{tg.Name}</div>{/each}
</aside>
