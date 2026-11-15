<script lang="ts">
  import { api } from '../lib/api'
  import { t, fail, updates, openURL } from '../lib/store.svelte'

  // "Sonra" denilen sürüm bir daha gösterilmez (yalnızca bu bilgisayarda hatırlanır).
  const key = 'gitpal.update.dismissed'
  let dismissed = $state('')
  try { dismissed = localStorage.getItem(key) ?? '' } catch { /* tarayıcı depolaması yoksa her seferinde gösterilir */ }

  const info = $derived(updates.info)
  const show = $derived(!!info?.available && (info.latest !== dismissed || updates.downloading || !!updates.hint))

  function later() {
    if (!info) return
    dismissed = info.latest
    updates.hint = ''
    try { localStorage.setItem(key, info.latest) } catch { /* yok sayılır */ }
  }

  async function download() {
    if (!info) return
    if (!info.hasPackage) { openURL(info.page); return }
    updates.downloading = true
    try {
      const d = await api.downloadUpdate()
      updates.hint = d.hint
    } catch (e) { fail(e) } finally { updates.downloading = false }
  }
</script>

{#if show && info}
  <div class="flex items-start gap-3 px-3 py-2 bg-emerald-50 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100 border-b border-emerald-200 dark:border-emerald-900">
    <span class="text-lg leading-none">⬆</span>
    <div class="flex-1 min-w-0">
      {#if updates.hint}
        <p class="whitespace-pre-wrap">{updates.hint}</p>
      {:else}
        <p><span class="font-semibold">{t('update.available')} gitpal {info.latest}</span>
          <span class="text-emerald-700 dark:text-emerald-300">({t('update.current')}: {info.current})</span></p>
      {/if}
    </div>
    {#if !updates.hint}
      <button class="btn-primary" disabled={updates.downloading} onclick={download}>
        {updates.downloading ? t('update.downloading') : t('update.download')}</button>
    {/if}
    <button class="btn" onclick={() => openURL(info.page)}>{t('update.notes')}</button>
    <button class="btn" onclick={later}>{updates.hint ? '✕' : t('update.later')}</button>
  </div>
{/if}
