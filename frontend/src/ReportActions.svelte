<script lang="ts">
  import { exportLastReport } from './lib/backend';

  let busy = false;
  let message = '';
  let error = '';

  async function save(format: 'text' | 'json') {
    if (busy) return;
    busy = true;
    message = '';
    error = '';
    try {
      const path = await exportLastReport(format);
      message = path ? `Saved report to ${path}` : 'Save cancelled.';
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="report-actions" aria-label="Verification report actions">
  <div class="report-copy">
    <strong>Verification report</strong>
    <span>Save the latest result as a readable text report or the stable JSON v1 schema.</span>
  </div>
  <div class="report-buttons">
    <button class="secondary" on:click={() => save('text')} disabled={busy}>Save TXT</button>
    <button class="secondary" on:click={() => save('json')} disabled={busy}>Save JSON</button>
  </div>
</div>
{#if message}<p class="report-status" role="status" aria-live="polite">{message}</p>{/if}
{#if error}<p class="report-status report-error" role="alert">{error}</p>{/if}

<style>
  .report-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    margin-top: 12px;
    padding: 11px 13px;
    border: 1px solid #29384a;
    border-radius: 9px;
    background: #101823;
  }
  .report-copy { display: grid; gap: 3px; min-width: 0; }
  .report-copy strong { color: #dbe6ef; font-size: 11px; }
  .report-copy span { color: #7f90a3; font-size: 10px; line-height: 1.4; }
  .report-buttons { display: flex; gap: 7px; flex: 0 0 auto; }
  .report-buttons button { min-width: 78px; }
  .report-status { margin: 7px 2px 0; color: #7fd6ba; font-size: 10px; overflow-wrap: anywhere; }
  .report-error { color: #ff9ca2; }
  @media (max-width: 760px) {
    .report-actions { align-items: stretch; flex-direction: column; }
    .report-buttons { width: 100%; }
    .report-buttons button { flex: 1; }
  }
</style>
