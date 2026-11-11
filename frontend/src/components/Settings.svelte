<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from '../lib/api'
  import { app, t, fail, notify, loadInfo } from '../lib/store.svelte'

  let name = $state('')
  let email = $state('')
  let global = $state(false)
  let aiKey = $state('')
  let hasKey = $state(false)
  let settings = $state<Record<string, string>>({})
  let edits = $state<Record<string, string>>({})

  onMount(async () => {
    try {
      const id = await api.identity()
      name = id.Name
      email = id.Email
      settings = await api.settings()
      edits = { ...settings }
      hasKey = (await api.aiStatus()).hasKey
    } catch (e) { fail(e) }
  })

  async function saveIdentity() {
    try { await api.setIdentity(name, email, global); notify(t('settings.saved')) } catch (e) { fail(e) }
  }
  async function saveKey() {
    try { await api.setAIKey(aiKey); aiKey = ''; hasKey = true; notify(t('settings.saved')) } catch (e) { fail(e) }
  }
  async function save(key: string) {
    try {
      await api.setSetting(key, edits[key])
      settings[key] = edits[key]
      notify(t('settings.saved'))
      if (key === 'ui.theme' || key === 'language') await loadInfo()
    } catch (e) { fail(e) }
  }
</script>

<div class="flex-1 overflow-auto p-4 space-y-8 max-w-3xl">
  <section class="space-y-2">
    <div class="panel-title px-0">{t('settings.identity')}</div>
    <label class="block">{t('settings.name')}<input class="input mt-1" bind:value={name} /></label>
    <label class="block">{t('settings.email')}<input class="input mt-1" bind:value={email} /></label>
    <label class="flex items-center gap-2 text-xs"><input type="checkbox" bind:checked={global} />{t('settings.global')}</label>
    <button class="btn-primary" onclick={saveIdentity}>{t('settings.save')}</button>
  </section>

  <section class="space-y-2">
    <div class="panel-title px-0">{t('settings.ai_key')}</div>
    {#if hasKey}<p class="text-xs text-emerald-600">{t('settings.key_set')}</p>{/if}
    <div class="flex gap-2">
      <input class="input mono" type="password" autocomplete="off" bind:value={aiKey} />
      <button class="btn-primary" disabled={!aiKey} onclick={saveKey}>{t('settings.save')}</button>
    </div>
  </section>

  <section class="space-y-1">
    <div class="panel-title px-0">{t('settings.all')}</div>
    {#each Object.keys(settings).sort() as key}
      <form class="flex items-center gap-2" onsubmit={(e) => { e.preventDefault(); save(key) }}>
        <span class="mono text-xs w-56 shrink-0 truncate" title={key}>{key}</span>
        <input class="input mono text-xs" bind:value={edits[key]} />
        <button class="btn text-xs" disabled={edits[key] === settings[key]}>{t('settings.save')}</button>
      </form>
    {/each}
  </section>
  <p class="text-xs text-zinc-400">{app.version}</p>
</div>
