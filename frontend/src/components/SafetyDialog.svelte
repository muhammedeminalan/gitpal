<script lang="ts">
  import { api, type Command } from '../lib/api'
  import { dialog, execute, t } from '../lib/store.svelte'

  let typed = $state('')
  let showCommands = $state(false)
  let commands = $state<Command[]>([])

  const a = $derived(dialog.assessment)
  const colors: Record<string, string> = {
    safe: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-100',
    caution: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-100',
    dangerous: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100',
    critical: 'bg-red-600 text-white',
  }
  const canConfirm = $derived(!!a && !a.blocked && (a.confirm !== 'type-branch' || typed === a.confirmText))

  function close(result: null | { approved: boolean }) {
    const op = dialog.op
    const resolve = dialog.resolve
    dialog.open = false
    const answer = typed
    typed = ''
    showCommands = false
    if (!resolve || !op) return
    if (!result) { resolve(null); return }
    execute(op, result.approved, answer).then(resolve)
  }

  async function toggleCommands() {
    showCommands = !showCommands
    if (showCommands) commands = (await api.commands()).slice(-8)
  }
</script>

{#if dialog.open && a}
  <div class="fixed inset-0 bg-black/40 grid place-items-center p-4 z-50" role="dialog" aria-modal="true">
    <div class="w-full max-w-lg rounded-lg bg-white dark:bg-zinc-900 shadow-xl p-5 space-y-4">
      <div class="flex items-center gap-2">
        <h2 class="text-lg font-semibold">{t('safety.title')}</h2>
        <span class="ml-auto px-2 py-0.5 rounded text-xs font-semibold {colors[a.risk]}">{t('safety.risk')}: {a.riskLabel}</span>
      </div>
      {#if a.blocked}<p class="text-red-600 font-medium">{t('safety.blocked')}</p>{/if}
      {#if a.effects.length}
        <div><div class="panel-title px-0">{t('safety.effects')}</div>
          <ul class="list-disc pl-5 space-y-1">{#each a.effects as e}<li>{e}</li>{/each}</ul></div>
      {/if}
      {#if a.findings.length}
        <div><div class="panel-title px-0">{t('safety.findings')}</div>
          <ul class="space-y-1">{#each a.findings as f}<li class="text-amber-700 dark:text-amber-300">! {f}</li>{/each}</ul></div>
      {/if}
      {#if a.backups.length}
        <div><div class="panel-title px-0">{t('safety.backups')}</div>
          {#each a.backups as b}<p class="text-zinc-500">{b}</p>{/each}</div>
      {/if}
      {#if a.confirm === 'type-branch' && !a.blocked}
        <label class="block">{t('safety.type')} <span class="mono font-semibold">{a.confirmText}</span>
          <input class="input mono mt-1" bind:value={typed} /></label>
      {/if}
      <button class="text-xs text-blue-600" onclick={toggleCommands}>{t('safety.commands')}</button>
      {#if showCommands}
        <pre class="mono text-xs bg-zinc-100 dark:bg-zinc-800 p-2 rounded overflow-auto max-h-40">{commands.map((c) => '$ ' + c.line).join('\n')}</pre>
      {/if}
      <div class="flex justify-end gap-2">
        <button class="btn" onclick={() => close(null)}>{t('safety.cancel')}</button>
        <button class={a.risk === 'dangerous' || a.risk === 'critical' ? 'px-3 py-1.5 rounded-md bg-red-600 text-white disabled:opacity-50' : 'btn-primary'}
          disabled={!canConfirm} onclick={() => close({ approved: true })}>{t('safety.confirm')}</button>
      </div>
    </div>
  </div>
{/if}
