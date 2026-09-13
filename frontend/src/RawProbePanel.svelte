<script lang="ts">
  import { onMount } from 'svelte';
  import {
    cancelRawProbe, onRawProbeProgress, prepareRawProbe, startRawProbe,
    type RawProbeChallenge, type RawProbeProgress, type RawProbeReport,
  } from './lib/backend';
  import {
    createRegionCells, formatBytes, rawRegionStateForProgress, updateRegion,
    type DeviceCard, type RegionCell,
  } from './lib/domain';

  export let device: DeviceCard;

  type RawState = 'idle' | 'armed' | 'running' | 'stopping' | 'success' | 'error';

  let state: RawState = 'idle';
  let challenge: RawProbeChallenge | undefined;
  let confirmation = '';
  let error = '';
  let report: RawProbeReport | undefined;
  let progress: RawProbeProgress | undefined;
  let regions: RegionCell[] = [];
  let currentDeviceId = '';

  $: if (device.id !== currentDeviceId && state !== 'running' && state !== 'stopping') {
    currentDeviceId = device.id;
    reset();
  }
  $: canArm = device.rawTest.allowed && state !== 'running' && state !== 'stopping';
  $: canStart = Boolean(challenge && confirmation === challenge.confirmationText && state === 'armed');

  function reset() {
    state = 'idle';
    challenge = undefined;
    confirmation = '';
    error = '';
    report = undefined;
    progress = undefined;
    regions = [];
  }

  function regionTitle(region: RegionCell): string {
    const base = `Sample ${region.index + 1}: ${region.state}`;
    return region.message ? `${base} — ${region.message}` : base;
  }

  function receiveProgress(next: RawProbeProgress) {
    progress = next;
    if (regions.length !== next.samplesTotal) regions = createRegionCells(next.samplesTotal);
    const regionState = rawRegionStateForProgress(next);
    if (regionState) regions = updateRegion(regions, next.sample, regionState, next.error);
  }

  async function arm() {
    if (!canArm) return;
    error = '';
    report = undefined;
    regions = [];
    progress = undefined;
    try {
      challenge = await prepareRawProbe(device.id);
      confirmation = '';
      state = 'armed';
    } catch (err) {
      state = 'error';
      error = err instanceof Error ? err.message : String(err);
    }
  }

  async function run() {
    if (!challenge || !canStart) return;
    const armed = challenge;
    state = 'running';
    error = '';
    report = undefined;
    regions = [];
    progress = undefined;
    try {
      report = await startRawProbe({
        deviceId: armed.deviceId,
        challengeToken: armed.token,
        confirmation,
        samples: 64,
        blockBytes: 4096,
      });
      state = 'success';
      challenge = undefined;
      confirmation = '';
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (message.toLowerCase().includes('canceled') || message.toLowerCase().includes('cancelled')) {
        state = 'idle';
      } else {
        state = 'error';
        error = message;
      }
    }
  }

  async function stop() {
    if (state !== 'running') return;
    state = 'stopping';
    try {
      const cancelled = await cancelRawProbe();
      if (!cancelled) state = 'idle';
    } catch (err) {
      state = 'error';
      error = err instanceof Error ? err.message : String(err);
    }
  }

  onMount(() => onRawProbeProgress(receiveProgress));
</script>

<section class="panel verification-panel raw-probe-panel">
  <div class="panel-title-row">
    <div><div class="eyebrow">Destructive test</div><h2>Raw capacity probe</h2></div>
    <span class="mode-badge raw-badge">Direct block writes</span>
  </div>

  <div class="raw-warning">
    <strong>Data-loss risk.</strong>
    <span>StorVerity saves and restores sampled blocks, but power loss, disconnects, fraudulent firmware, or restoration errors can still destroy data. Use only expendable media.</span>
  </div>

  {#if !device.rawTest.allowed}
    <div class="protected-state raw-protected">
      <div class="shield-icon"><svg viewBox="0 0 32 32"><path d="M16 4 26 8v7c0 6.5-4 10.5-10 13-6-2.5-10-6.5-10-13V8l10-4Z"/><path d="M11 11l10 10M21 11 11 21"/></svg></div>
      <div>
        <strong>Raw probing blocked</strong>
        <p>The whole device must be external, writable, unmounted, non-system storage, and free of swap. Safety is checked again immediately before opening it.</p>
        {#if device.rawTest.reasons?.length}
          <ul class="reason-list">
            {#each device.rawTest.reasons as reason}<li>{reason.message}</li>{/each}
          </ul>
        {/if}
      </div>
    </div>
  {:else}
    {#if state === 'armed' && challenge}
      <div class="confirmation-box">
        <strong>Confirm target: {challenge.displayName}</strong>
        <p>{challenge.path} · {formatBytes(challenge.capacityBytes)}. Type the exact phrase below. The challenge expires automatically and is valid for one run only.</p>
        <code>{challenge.confirmationText}</code>
        <input bind:value={confirmation} autocomplete="off" spellcheck="false" aria-label="Raw probe confirmation" placeholder="Type the confirmation phrase" />
      </div>
    {/if}

    {#if regions.length > 0}
      <div class="map-frame raw-map">
        <div class="map-header">
          <div><strong>{progress ? `Raw probe · ${progress.phase}` : 'Raw sample map'}</strong><span>{regions.length} sampled locations across the advertised address space</span></div>
          <div class="progress-number">{progress ? `${progress.sample + 1}/${progress.samplesTotal}` : ''}</div>
        </div>
        <div class="region-grid" aria-label="Raw capacity sample map">
          {#each regions as region}<span class={`region ${region.state}`} title={regionTitle(region)}></span>{/each}
        </div>
        <div class="legend"><span><i class="pending"></i>Pending</span><span><i class="writing"></i>Written</span><span><i class="valid"></i>Verified</span><span><i class="corrupt"></i>Mismatch / I/O error</span></div>
      </div>
    {/if}

    {#if error}<div class="alert error"><strong>Raw probe failed</strong><span>{error}</span></div>{/if}
    {#if state === 'success' && report}
      <div class:raw-danger={report.suspectFakeCapacity || report.restoreErrors > 0} class="raw-result">
        <strong>{report.suspectFakeCapacity ? 'Capacity is suspicious' : 'Sampled capacity validated'}</strong>
        <span>{report.validSamples}/{report.samples} samples verified · {report.corruptSamples} mismatches · {report.readErrors} read errors · {report.writeErrors} write errors · {report.restoreErrors} restore errors.</span>
        <small>{report.restored ? 'All touched samples reported restored.' : 'Restoration was incomplete. Do not trust the device contents.'}</small>
      </div>
    {/if}

    <div class="action-row raw-actions">
      <p>This is a sampled fraud detector, not an exhaustive media scan. It never accepts a manually entered device path.</p>
      {#if state === 'running'}
        <button class="secondary danger-button" on:click={stop}>Stop and restore</button>
      {:else if state === 'stopping'}
        <button class="secondary" disabled>Restoring…</button>
      {:else if state === 'armed'}
        <div class="button-pair">
          <button class="secondary" on:click={reset}>Cancel</button>
          <button class="primary destructive" on:click={run} disabled={!canStart}>Start raw probe</button>
        </div>
      {:else}
        <button class="secondary danger-button" on:click={arm} disabled={!canArm}>Prepare raw probe</button>
      {/if}
    </div>
  {/if}
</section>

<style>
  .raw-probe-panel { margin-top: 13px; }
  .raw-badge { border-color: #633b42; background: #28181d; color: #f29ba1; }
  .raw-warning { display: grid; grid-template-columns: auto 1fr; gap: 8px 12px; margin: 18px 0 14px; padding: 12px 14px; border: 1px solid #5b363d; border-radius: 10px; background: #23161a; font-size: 10.5px; line-height: 1.5; }
  .raw-warning strong { color: #ff9da3; white-space: nowrap; }
  .raw-warning span { color: #b9a0a4; }
  .raw-protected { margin-top: 0; }
  .reason-list { margin: 9px 0 0; padding-left: 18px; color: #8b99ab; font-size: 10px; line-height: 1.5; }
  .confirmation-box { margin: 14px 0; padding: 14px; border: 1px solid #49363d; border-radius: 10px; background: #111720; }
  .confirmation-box strong, .confirmation-box code { display: block; }
  .confirmation-box p { margin: 5px 0 10px; color: #7c8b9e; font-size: 10.5px; line-height: 1.5; }
  .confirmation-box code { width: fit-content; margin-bottom: 10px; padding: 5px 7px; border-radius: 5px; background: #27181d; color: #ffa5aa; font-size: 10px; }
  .confirmation-box input { width: 100%; height: 39px; border: 1px solid #493841; border-radius: 9px; outline: 0; background: #0d1520; color: #e0e5eb; padding: 0 11px; font: 11px ui-monospace, SFMono-Regular, Menlo, monospace; }
  .confirmation-box input:focus { border-color: #8a4d58; box-shadow: 0 0 0 3px #ff767c12; }
  .raw-map { margin: 14px 0; }
  :global(.region.restore-error) { border-color: #c45d36; background: #974724; }
  .raw-result { display: grid; gap: 4px; margin-top: 13px; padding: 11px 13px; border: 1px solid #285248; border-radius: 9px; background: #11251f; color: #7fe0bd; font-size: 10.5px; }
  .raw-result span, .raw-result small { color: #9eabbc; }
  .raw-result.raw-danger { border-color: #65373e; background: #26161b; color: #ff969c; }
  .raw-actions { margin-top: 14px; }
  .button-pair { display: flex; gap: 8px; }
  .destructive { border-color: #ff8f95; background: linear-gradient(#ff969c, #e96f76); color: #2b0d10; box-shadow: 0 8px 24px #ff767c22; }
</style>
