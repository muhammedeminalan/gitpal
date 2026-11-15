<script lang="ts">
  import { branchDialog, createBranch, t } from '../lib/store.svelte'

  let name = $state('')
  let input = $state<HTMLInputElement | null>(null)

  // Pencere açılınca ad kutusuna odaklan.
  $effect(() => { if (branchDialog.open) { name = ''; queueMicrotask(() => input?.focus()) } })

  async function submit(e: Event) {
    e.preventDefault()
    const n = name.trim()
    if (!n) return
    // Ad geçersizse hata (ve düzeltilmiş ad önerisi) alt şeritte görünür; pencere açık kalır.
    if (await createBranch(n)) branchDialog.open = false
  }
</script>

{#if branchDialog.open}
  <div class="fixed inset-0 bg-black/40 grid place-items-center p-4 z-50" role="dialog" aria-modal="true" tabindex="-1"
    onkeydown={(e) => e.key === 'Escape' && (branchDialog.open = false)}>
    <form class="w-full max-w-md rounded-lg bg-white dark:bg-zinc-900 shadow-xl p-5 space-y-4" onsubmit={submit}>
      <h2 class="text-lg font-semibold">{t('branch.new_title')}</h2>
      <p class="text-sm text-zinc-500">{t('branch.new_hint')}</p>
      <label class="block">{t('branch.name')}
        <input class="input mono mt-1" placeholder={t('branch.placeholder')} bind:value={name} bind:this={input} />
      </label>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn" onclick={() => (branchDialog.open = false)}>{t('safety.cancel')}</button>
        <button type="submit" class="btn-primary" disabled={!name.trim()}>{t('branch.create')}</button>
      </div>
    </form>
  </div>
{/if}
