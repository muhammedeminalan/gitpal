<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type GuideGroup } from '../lib/api'
  import { t, fail, notify } from '../lib/store.svelte'

  let { onterminal }: { onterminal: () => void } = $props()
  let groups = $state<GuideGroup[]>([])
  let query = $state('')
  let copied = $state('')

  onMount(() => { api.guide().then((g) => (groups = g)).catch(fail) })

  const q = $derived(query.trim().toLocaleLowerCase('tr'))
  const shown = $derived(groups
    .map((g) => ({ ...g, entries: g.entries.filter((e) => !q || (e.example + ' ' + e.text).toLocaleLowerCase('tr').includes(q)) }))
    .filter((g) => g.entries.length > 0))

  async function copy(text: string) {
    try {
      // Wails penceresinde sistem panosu runtime üzerinden; tarayıcıda navigator.clipboard.
      const rt = (window as unknown as { runtime?: { ClipboardSetText?: (s: string) => Promise<boolean> } }).runtime
      if (rt?.ClipboardSetText) await rt.ClipboardSetText(text)
      else await navigator.clipboard.writeText(text)
      copied = text
      notify(t('help.copied') + ': ' + text)
      setTimeout(() => { if (copied === text) copied = '' }, 1500)
    } catch (e) { fail(e) }
  }
</script>

<div class="flex-1 overflow-auto p-4">
  <div class="max-w-5xl mx-auto space-y-4">
    <div class="flex items-start gap-4">
      <div class="flex-1">
        <h2 class="text-lg font-semibold">{t('help.title')}</h2>
        <p class="text-zinc-500">{t('help.intro')}</p>
      </div>
      <button class="btn-primary shrink-0" onclick={onterminal}>⌨ {t('toolbar.terminal')}</button>
    </div>
    <input class="input" placeholder={t('help.search')} bind:value={query} />
    {#each shown as g (g.title)}
      <section>
        <div class="panel-title px-0">{g.title}</div>
        <table class="w-full text-sm border-collapse">
          <thead class="sr-only"><tr><th>{t('help.example')}</th><th>{t('help.what')}</th><th></th></tr></thead>
          <tbody>
            {#each g.entries as e (e.example)}
              <tr class="border-b border-zinc-100 dark:border-zinc-800 hover:bg-zinc-50 dark:hover:bg-zinc-800/50">
                <td class="py-1.5 pr-4 align-top w-[45%]"><code class="mono text-emerald-700 dark:text-emerald-400 break-words">{e.example}</code></td>
                <td class="py-1.5 pr-2 align-top">{e.text}</td>
                <td class="py-1.5 align-top text-right w-24">
                  <button class="btn text-xs py-0.5" onclick={() => copy(e.example)}>{copied === e.example ? '✓ ' + t('help.copied') : t('help.copy')}</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/each}
    {#if groups.length && !shown.length}<p class="text-zinc-500">{t('help.none')}</p>{/if}
  </div>
</div>
