<script lang="ts">
  import { onMount } from 'svelte'
  import { app, t, loadInfo, refresh, fail, guarded, notify } from './lib/store.svelte'
  import { api } from './lib/api'
  import Welcome from './components/Welcome.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Changes from './components/Changes.svelte'
  import History from './components/History.svelte'
  import Operations from './components/Operations.svelte'
  import Settings from './components/Settings.svelte'
  import SafetyDialog from './components/SafetyDialog.svelte'
  import Palette from './components/Palette.svelte'

  type Tab = 'changes' | 'history' | 'operations' | 'settings'
  let tab = $state<Tab>('changes')
  let paletteOpen = $state(false)
  let ready = $state(false)

  onMount(() => {
    loadInfo().then(() => (ready = true)).catch(fail)
    // Keep the view current while the window is in front: the user may run
    // git elsewhere too.
    const timer = setInterval(() => { if (document.hasFocus() && !app.busy) refresh() }, 3000)
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const onTheme = () => loadInfo().catch(fail)
    media.addEventListener('change', onTheme)
    return () => { clearInterval(timer); media.removeEventListener('change', onTheme) }
  })

  function onKey(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      paletteOpen = !paletteOpen
    }
  }

  async function opened(root: string) {
    app.root = root
    await refresh()
  }

  async function sync(kind: 'fetch' | 'pull' | 'push') {
    if (kind === 'fetch') {
      app.busy = true
      try { await api.fetch(); notify(t('toolbar.fetch') + ' ✓') } catch (e) { fail(e) } finally { app.busy = false; refresh() }
      return
    }
    await guarded({ kind })
  }

  const actions = $derived([
    { label: t('toolbar.fetch'), run: () => sync('fetch') },
    { label: t('toolbar.pull'), run: () => sync('pull') },
    { label: t('toolbar.push'), run: () => sync('push') },
    { label: t('toolbar.refresh'), run: () => refresh() },
    { label: t('files.stage_all'), run: () => api.stageAll().then(refresh).catch(fail) },
    { label: t('nav.changes'), run: () => (tab = 'changes') },
    { label: t('nav.history'), run: () => (tab = 'history') },
    { label: t('nav.operations'), run: () => (tab = 'operations') },
    { label: t('nav.settings'), run: () => (tab = 'settings') },
    { label: t('ops.undo'), run: () => (tab = 'operations') },
    { label: t('nav.switch_repo'), run: () => { app.root = ''; app.state = null } },
  ])

  const branch = $derived(app.state?.branch)
</script>

<svelte:window onkeydown={onKey} />

{#if !ready}
  <div class="h-full grid place-items-center text-zinc-500">…</div>
{:else if !app.root}
  <Welcome onopen={opened} />
{:else}
  <div class="h-full flex flex-col">
    <header class="flex items-center gap-3 px-3 py-2 border-b border-zinc-200 dark:border-zinc-700">
      <span class="flex items-center gap-2 font-semibold"><img src="/gitpal-logo.svg" alt="" class="w-6 h-6" />gitpal</span>
      <span class="mono text-zinc-500 truncate" title={app.root}>{app.root.split(/[\\/]/).pop()}</span>
      {#if branch}
        <span class="mono px-2 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800">
          {branch.Detached ? t('status.detached') : branch.Head}
          {#if app.state?.protected}<span class="text-amber-600"> · {t('status.protected')}</span>{/if}
        </span>
        {#if branch.Unborn}
          <span class="text-zinc-500">{t('status.unborn')}</span>
        {:else if branch.Upstream && !branch.UpstreamGone}
          <span class="text-zinc-500">↑{branch.Ahead} ↓{branch.Behind}</span>
        {:else if !branch.Detached}
          <span class="text-zinc-500">{t('status.no_upstream')}</span>
        {/if}
      {/if}
      {#if app.state?.operation}
        <span class="px-2 py-0.5 rounded bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100">{app.state.operation}</span>
        <button class="btn-primary" disabled={app.state.files.some((f) => f.conflicted)} onclick={() => guarded({ kind: 'continue' })}>{t('toolbar.continue')}</button>
        {#if app.state.operation !== 'merge'}<button class="btn" onclick={() => guarded({ kind: 'skip' })}>{t('toolbar.skip')}</button>{/if}
        <button class="btn" onclick={() => guarded({ kind: 'abort' })}>{t('toolbar.abort')}</button>
      {/if}
      <div class="ml-auto flex gap-2">
        <button class="btn" disabled={app.busy} onclick={() => sync('fetch')}>{t('toolbar.fetch')}</button>
        <button class="btn" disabled={app.busy} onclick={() => sync('pull')}>{t('toolbar.pull')}</button>
        <button class="btn" disabled={app.busy} onclick={() => sync('push')}>{t('toolbar.push')}</button>
        <button class="btn" title={t('toolbar.palette')} onclick={() => (paletteOpen = true)}>⌘K</button>
      </div>
    </header>
    <nav class="flex gap-1 px-3 border-b border-zinc-200 dark:border-zinc-700">
      {#each [['changes', 'nav.changes'], ['history', 'nav.history'], ['operations', 'nav.operations'], ['settings', 'nav.settings']] as [id, key]}
        <button class="px-3 py-2 border-b-2 {tab === id ? 'border-blue-600 font-medium' : 'border-transparent text-zinc-500'}"
          onclick={() => (tab = id as Tab)}>{t(key)}</button>
      {/each}
    </nav>
    <main class="flex-1 min-h-0 flex">
      {#if tab === 'changes'}
        <Sidebar />
        <Changes />
      {:else if tab === 'history'}
        <History />
      {:else if tab === 'operations'}
        <Operations />
      {:else}
        <Settings />
      {/if}
    </main>
    {#if app.error}
      <div class="px-3 py-2 bg-red-50 text-red-800 dark:bg-red-950 dark:text-red-200 whitespace-pre-wrap flex gap-2">
        <span class="flex-1">{app.error}</span><button onclick={() => (app.error = '')}>✕</button>
      </div>
    {/if}
    {#if app.toast}
      <div class="px-3 py-2 bg-emerald-50 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">{app.toast}</div>
    {/if}
  </div>
{/if}

<SafetyDialog />
{#if paletteOpen}
  <Palette {actions} onclose={() => (paletteOpen = false)} />
{/if}
