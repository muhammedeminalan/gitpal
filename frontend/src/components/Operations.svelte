<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type Backup, type Command, type HistoryEntry, type UndoInfo } from '../lib/api'
  import { app, t, fail, notify, refresh } from '../lib/store.svelte'

  let undo = $state<UndoInfo | null>(null)
  let entries = $state<HistoryEntry[]>([])
  let backups = $state<Backup[]>([])
  let commands = $state<Command[]>([])
  let confirming = $state(false)

  async function load() {
    try {
      ;[undo, entries, backups, commands] = await Promise.all([api.planUndo(), api.historyEntries(), api.backups(), api.commands()])
    } catch (e) { fail(e) }
  }
  onMount(load)

  async function doUndo() {
    confirming = false
    app.busy = true
    try { await api.undo(); notify(t('ops.undo') + ' ✓') } catch (e) { fail(e) } finally { app.busy = false; await refresh(); load() }
  }
</script>

<div class="flex-1 overflow-auto p-4 grid gap-6 lg:grid-cols-2 content-start">
  <section class="space-y-2">
    <div class="panel-title px-0">{t('ops.undo')}</div>
    {#if undo?.possible}
      <p><span class="font-medium">{undo.op}</span> <span class="text-zinc-500">· {new Date(undo.when).toLocaleString()}</span></p>
      {#if confirming}
        <div class="flex gap-2">
          <button class="btn" onclick={() => (confirming = false)}>{t('safety.cancel')}</button>
          <button class="btn-primary" disabled={app.busy} onclick={doUndo}>{t('safety.confirm')}</button>
        </div>
      {:else}
        <button class="btn-primary" onclick={() => (confirming = true)}>{t('ops.undo')}</button>
      {/if}
    {:else}
      <p class="text-zinc-500">{t('ops.undo_none')}{undo?.reason ? `: ${undo.reason}` : ''}</p>
    {/if}
  </section>

  <section class="space-y-1">
    <div class="panel-title px-0">{t('ops.backups')}</div>
    {#each backups as b (b.Ref)}
      <div class="mono text-xs truncate" title={b.Ref}>{new Date(b.Time).toLocaleString()} · {b.Op} · {b.Hash.slice(0, 7)}{b.Worktree ? ' · worktree' : ''}</div>
    {/each}
  </section>

  <section class="space-y-1">
    <div class="panel-title px-0">{t('ops.history')}</div>
    {#each entries as e (e.id)}
      <div class="text-xs flex gap-2">
        <span class="text-zinc-500 shrink-0">{new Date(e.time).toLocaleString()}</span>
        <span class="font-medium">{e.op}</span>
        {#if e.branch}<span class="mono">{e.branch}</span>{/if}
        {#if e.failed}<span class="text-red-600">{t('ops.failed')}</span>{/if}
      </div>
    {/each}
  </section>

  <section class="space-y-1">
    <div class="panel-title px-0">{t('ops.commands')}</div>
    <pre class="mono text-xs bg-zinc-100 dark:bg-zinc-800 p-2 rounded overflow-auto max-h-96">{commands.map((c) => `$ ${c.line}  (${c.exitCode}, ${c.millis}ms)`).join('\n')}</pre>
  </section>
</div>
