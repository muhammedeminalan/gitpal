<script lang="ts">
  import { api, type Branch } from '../lib/api'
  import { app, t, fail, branchDialog, switchBranch } from '../lib/store.svelte'

  let open = $state(false)
  let query = $state('')
  let branches = $state<Branch[]>([])

  const branch = $derived(app.state?.branch)
  const q = $derived(query.trim().toLocaleLowerCase('tr'))
  const match = (b: Branch) => !q || b.Name.toLocaleLowerCase('tr').includes(q)
  const local = $derived(branches.filter((b) => !b.Remote && match(b)))
  // Uzak branch'lerden yerelde karşılığı olanlar gösterilmez (aynı şey iki kez görünmesin).
  const remote = $derived(branches.filter((b) => b.Remote && !b.Name.endsWith('/HEAD') && match(b) &&
    !branches.some((l) => !l.Remote && l.Name === shortName(b.Name))))

  function shortName(remoteName: string) { return remoteName.replace(/^[^/]+\//, '') }

  async function toggle() {
    open = !open
    if (!open) return
    query = ''
    try { branches = await api.branches() } catch (e) { fail(e) }
  }

  function go(name: string) {
    open = false
    switchBranch(name)
  }
</script>

<div class="relative">
  <button class="mono px-2 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 hover:bg-zinc-200 dark:hover:bg-zinc-700 flex items-center gap-1"
    title={t('branch.menu')} onclick={toggle}>
    {branch?.Detached ? t('status.detached') : branch?.Head}
    {#if app.state?.protected}<span class="text-amber-600"> · {t('status.protected')}</span>{/if}
    <span class="text-zinc-400 text-xs">▾</span>
  </button>
  {#if open}
    <!-- Menünün dışına tıklayınca kapanır. -->
    <button class="fixed inset-0 z-30 cursor-default" aria-label="close" onclick={() => (open = false)}></button>
    <div class="absolute left-0 top-full mt-1 z-40 w-80 rounded-lg border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 shadow-xl">
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input border-0 border-b rounded-none rounded-t-lg px-3 py-2" autofocus placeholder={t('branch.search')} bind:value={query}
        onkeydown={(e) => { if (e.key === 'Escape') open = false; if (e.key === 'Enter' && local[0] && !local[0].Current) go(local[0].Name) }} />
      <div class="max-h-80 overflow-auto py-1">
        <div class="panel-title">{t('branch.local')}</div>
        {#each local as b (b.Ref)}
          <button class="w-full text-left px-3 py-1.5 flex items-center gap-2 {b.Current ? 'cursor-default' : 'hover:bg-blue-50 dark:hover:bg-blue-950'}"
            title={b.Current ? '' : t('branch.switch_tip')} disabled={b.Current} onclick={() => go(b.Name)}>
            <span class="w-3 text-blue-600">{b.Current ? '✓' : ''}</span>
            <span class="mono truncate flex-1 {b.Current ? 'font-semibold' : ''}">{b.Name}</span>
            {#if b.Current}<span class="text-xs text-zinc-400">{t('branch.current')}</span>{/if}
          </button>
        {/each}
        {#if remote.length}
          <div class="panel-title mt-2">{t('branch.remote')}</div>
          {#each remote as b (b.Ref)}
            <button class="w-full text-left px-3 py-1.5 flex items-center gap-2 hover:bg-blue-50 dark:hover:bg-blue-950 text-zinc-600 dark:text-zinc-300"
              title={t('branch.switch_tip')} onclick={() => go(shortName(b.Name))}>
              <span class="w-3"></span><span class="mono truncate">{b.Name}</span>
            </button>
          {/each}
        {/if}
        {#if !local.length && !remote.length}<p class="px-3 py-2 text-zinc-500">{t('branch.none')}</p>{/if}
      </div>
      <button class="w-full text-left px-3 py-2 border-t border-zinc-200 dark:border-zinc-700 text-blue-600 hover:bg-zinc-50 dark:hover:bg-zinc-800 rounded-b-lg"
        onclick={() => { open = false; branchDialog.open = true }}>＋ {t('branch.new')}</button>
    </div>
  {/if}
</div>
