<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from '../lib/api'
  import { app, t, fail } from '../lib/store.svelte'

  let { onopen }: { onopen: (root: string) => void } = $props()
  let recent = $state<string[]>([])
  let path = $state('')
  let url = $state('')

  onMount(() => { api.recentRepos().then((r) => (recent = r)).catch(fail) })

  async function open(fn: () => Promise<string>) {
    app.error = ''
    try { onopen(await fn()) } catch (e) { fail(e) }
  }
</script>

<div class="h-full grid place-items-center p-6">
  <div class="w-full max-w-xl space-y-6">
    <h1 class="text-2xl font-semibold">{t('welcome.title')}</h1>
    <section class="space-y-2">
      <div class="panel-title px-0">{t('welcome.recent')}</div>
      {#if recent.length === 0}<p class="text-zinc-500">{t('welcome.none')}</p>{/if}
      {#each recent as r}
        <button class="block w-full text-left mono px-3 py-2 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800" onclick={() => open(() => api.openRepo(r))}>{r}</button>
      {/each}
    </section>
    <section class="space-y-2">
      <label class="block">{t('welcome.path')}
        <div class="flex gap-2 mt-1">
          <input class="input mono" bind:value={path} />
          <button class="btn shrink-0" onclick={async () => { try { const d = await api.pickFolder(); if (d) path = d } catch (e) { fail(e) } }}>{t('welcome.browse')}</button>
        </div>
      </label>
      <div class="flex gap-2">
        <button class="btn-primary" disabled={!path} onclick={() => open(() => api.openRepo(path))}>{t('welcome.open')}</button>
        <button class="btn" disabled={!path} onclick={() => open(() => api.initRepo(path))}>{t('welcome.init')}</button>
      </div>
    </section>
    <section class="space-y-2">
      <label class="block">{t('welcome.url')}<input class="input mono mt-1" bind:value={url} /></label>
      <button class="btn" disabled={!url || !path} onclick={() => open(() => api.cloneRepo(url, path))}>{t('welcome.clone')}</button>
    </section>
    {#if app.error}<p class="text-red-600 whitespace-pre-wrap">{app.error}</p>{/if}
    <p class="text-xs text-zinc-400">{app.version}</p>
  </div>
</div>
