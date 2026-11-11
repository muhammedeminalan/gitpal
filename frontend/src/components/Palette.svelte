<script lang="ts">
  import { t } from '../lib/store.svelte'

  let { actions, onclose }: { actions: { label: string; run: () => unknown }[]; onclose: () => void } = $props()
  let query = $state('')
  let index = $state(0)
  const shown = $derived(actions.filter((a) => a.label.toLowerCase().includes(query.toLowerCase())))

  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose()
    else if (e.key === 'ArrowDown') index = Math.min(index + 1, shown.length - 1)
    else if (e.key === 'ArrowUp') index = Math.max(index - 1, 0)
    else if (e.key === 'Enter' && shown[index]) { onclose(); shown[index].run() }
  }
</script>

<div class="fixed inset-0 bg-black/30 z-40 pt-24" role="presentation" onclick={onclose}>
  <div class="mx-auto max-w-md rounded-lg bg-white dark:bg-zinc-900 shadow-xl" role="dialog" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={key}>
    <!-- svelte-ignore a11y_autofocus -->
    <input class="input border-0 border-b rounded-none px-3 py-2" autofocus placeholder={t('palette.placeholder')} bind:value={query} oninput={() => (index = 0)} />
    <ul class="max-h-80 overflow-auto py-1">
      {#each shown as a, i}
        <li><button class="w-full text-left px-3 py-1.5 {i === index ? 'bg-blue-600 text-white' : ''}" onclick={() => { onclose(); a.run() }}>{a.label}</button></li>
      {/each}
    </ul>
  </div>
</div>
