<script lang="ts">
  import { api, type AIStatus, type Suggestion } from '../lib/api'
  import { app, t, fail, guarded } from '../lib/store.svelte'

  let message = $state('')
  let amend = $state(false)
  let ai = $state<AIStatus | null>(null)
  let consentOpen = $state(false)
  let suggestion = $state<Suggestion | null>(null)
  let thinking = $state(false)

  const stagedCount = $derived(app.state?.files.filter((f) => f.staged).length ?? 0)
  const canCommit = $derived(!app.busy && message.trim() !== '' && (stagedCount > 0 || amend))

  async function commit(push: boolean) {
    if (!canCommit) return
    const out = await guarded({ kind: amend ? 'amend' : 'commit', message, amend })
    if (!out) return
    message = ''
    amend = false
    suggestion = null
    if (push) await guarded({ kind: 'push' })
  }

  async function generate() {
    try {
      ai = await api.aiStatus()
      if (!ai.consent) { consentOpen = true; return }
      thinking = true
      suggestion = await api.suggestCommit()
      if (suggestion.messages.length) message = suggestion.messages[0]
    } catch (e) { fail(e) } finally { thinking = false }
  }

  async function agree() {
    consentOpen = false
    try { await api.giveAIConsent(); await generate() } catch (e) { fail(e) }
  }

  function onKey(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); commit(false) }
  }
</script>

<div class="border-t border-zinc-200 dark:border-zinc-700 p-3 space-y-2">
  <textarea class="input mono h-24 resize-none" placeholder={t('commit.placeholder')} bind:value={message} onkeydown={onKey}></textarea>
  {#if suggestion}
    {#each suggestion.messages.slice(1) as m}
      <button class="block w-full text-left mono text-xs truncate text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100" onclick={() => (message = m)}>↳ {m}</button>
    {/each}
    {#if suggestion.split}<p class="text-xs text-amber-600">{t('ai.split')}: {suggestion.split}</p>{/if}
    {#if suggestion.withheld?.length}<p class="text-xs text-zinc-500">{t('ai.withheld')}: {suggestion.withheld.join(', ')}</p>{/if}
  {/if}
  <label class="flex items-center gap-2 text-xs"><input type="checkbox" bind:checked={amend} />{t('commit.amend')}</label>
  <div class="flex gap-2">
    <button class="btn-primary flex-1" disabled={!canCommit} onclick={() => commit(false)}>{t('commit.commit')}</button>
    <button class="btn" disabled={!canCommit} onclick={() => commit(true)}>{t('commit.push')}</button>
    <button class="btn" disabled={thinking || stagedCount === 0} onclick={generate}>{thinking ? '…' : t('commit.ai')}</button>
  </div>
</div>

{#if consentOpen && ai}
  <div class="fixed inset-0 bg-black/40 grid place-items-center p-4 z-50" role="dialog" aria-modal="true">
    <div class="w-full max-w-lg rounded-lg bg-white dark:bg-zinc-900 shadow-xl p-5 space-y-4">
      <h2 class="text-lg font-semibold">{t('ai.consent_title')}</h2>
      <p class="whitespace-pre-wrap">{ai.notice}</p>
      <div class="flex justify-end gap-2">
        <button class="btn" onclick={() => (consentOpen = false)}>{t('safety.cancel')}</button>
        <button class="btn-primary" onclick={agree}>{t('ai.agree')}</button>
      </div>
    </div>
  </div>
{/if}
