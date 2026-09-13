<script lang="ts">
  import { onMount } from 'svelte';
  import {
    cancelVerification, listDevices, onVerificationProgress, startVerification,
    type VerificationProgress, type VerificationReport,
  } from './lib/backend';
  import {
    canVerifyFilesystem, createRegionCells, formatBytes, overallProgress,
    safetyLabel, updateRegion, type DeviceCard, type RegionCell,
  } from './lib/domain';

  type RunState = 'idle' | 'running' | 'stopping' | 'success' | 'error';

  const MiB = 1024 * 1024;
  const GiB = 1024 * MiB;
  const testSizes = [
    { label: 'Quick', value: 256 * MiB, detail: '256 MiB' },
    { label: 'Standard', value: 1 * GiB, detail: '1 GiB' },
    { label: 'Extended', value: 4 * GiB, detail: '4 GiB' },
  ];

  let devices: DeviceCard[] = [];
  let selectedId = '';
  let selectedMount = '';
  let loading = true;
  let loadError = '';
  let runError = '';
  let runState: RunState = 'idle';
  let testBytes = testSizes[0].value;
  let regions: RegionCell[] = [];
  let progress: VerificationProgress | undefined;
  let report: VerificationReport | undefined;
  let startedAt = 0;
  let elapsedSeconds = 0;
  let timer: ReturnType<typeof setInterval> | undefined;

  $: selected = devices.find((device) => device.id === selectedId);
  $: verifyEnabled = canVerifyFilesystem(selected) && runState !== 'running' && runState !== 'stopping';
  $: progressValue = overallProgress(progress);
  $: progressPercent = Math.round(progressValue * 100);

  function chooseDevice(device: DeviceCard) {
    selectedId = device.id;
    selectedMount = device.mountPoints[0] ?? '';
    resetResult();
  }

  function resetResult() {
    if (runState === 'running' || runState === 'stopping') return;
    runState = 'idle';
    runError = '';
    progress = undefined;
    report = undefined;
    regions = [];
  }

  async function refresh() {
    loading = true;
    loadError = '';
    try {
      const next = await listDevices();
      devices = next;
      const stillPresent = next.some((device) => device.id === selectedId);
      if (!stillPresent) {
        const preferred = next.find((device) => device.likelyExternal) ?? next[0];
        selectedId = preferred?.id ?? '';
        selectedMount = preferred?.mountPoints[0] ?? '';
      }
    } catch (error) {
      loadError = error instanceof Error ? error.message : String(error);
    } finally {
      loading = false;
    }
  }

  function receiveProgress(next: VerificationProgress) {
    progress = next;
    if (regions.length !== next.regionsTotal) regions = createRegionCells(next.regionsTotal);
    regions = updateRegion(regions, next.region, next.phase === 'write' ? 'writing' : 'valid');
  }

  function startTimer() {
    startedAt = Date.now();
    elapsedSeconds = 0;
    if (timer) clearInterval(timer);
    timer = setInterval(() => {
      elapsedSeconds = Math.floor((Date.now() - startedAt) / 1000);
    }, 500);
  }

  function stopTimer() {
    if (timer) clearInterval(timer);
    timer = undefined;
    if (startedAt) elapsedSeconds = Math.floor((Date.now() - startedAt) / 1000);
  }

  async function runVerification() {
    if (!selected || !selectedMount || !canVerifyFilesystem(selected)) return;
    runState = 'running';
    runError = '';
    report = undefined;
    progress = undefined;
    regions = [];
    startTimer();
    try {
      report = await startVerification({
        deviceId: selected.id,
        mountPoint: selectedMount,
        totalBytes: testBytes,
        chunkBytes: 16 * MiB,
      });
      runState = 'success';
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      const lowered = message.toLowerCase();
      if (lowered.includes('canceled') || lowered.includes('cancelled')) {
        runState = 'idle';
      } else {
        runState = 'error';
        runError = message;
      }
    } finally {
      stopTimer();
      await refresh();
    }
  }

  async function stopVerification() {
    if (runState !== 'running') return;
    runState = 'stopping';
    try {
      const cancelled = await cancelVerification();
      if (!cancelled) runState = 'idle';
    } catch (error) {
      runState = 'error';
      runError = error instanceof Error ? error.message : String(error);
    }
  }

  onMount(() => {
    const unsubscribe = onVerificationProgress(receiveProgress);
    void refresh();
    return () => {
      unsubscribe();
      if (timer) clearInterval(timer);
    };
  });
</script>

<div class="app-shell">
  <aside class="sidebar">
    <header class="brand">
      <div class="brand-mark" aria-hidden="true">
        <svg viewBox="0 0 32 32"><path d="M8 5.5h16a3 3 0 0 1 3 3v15a3 3 0 0 1-3 3H8a3 3 0 0 1-3-3v-15a3 3 0 0 1 3-3Z"/><path d="M9.5 10h13M10 21.5h.01M14 21.5h8"/></svg>
      </div>
      <div><strong>StorVerity</strong><span>Storage integrity</span></div>
    </header>

    <div class="sidebar-heading">
      <span>Storage devices</span>
      <button class="icon-button" on:click={refresh} disabled={loading} aria-label="Refresh devices" title="Refresh devices">
        <svg viewBox="0 0 24 24"><path d="M20 7v5h-5M4 17v-5h5M6.1 9a7 7 0 0 1 11.6-2.6L20 8M4 16l2.3 1.6A7 7 0 0 0 18 15"/></svg>
      </button>
    </div>

    <div class="device-list" aria-live="polite">
      {#if loading && devices.length === 0}
        <div class="sidebar-state"><span class="spinner"></span><p>Scanning block devices…</p></div>
      {:else if loadError}
        <div class="sidebar-state error"><p>{loadError}</p><button on:click={refresh}>Try again</button></div>
      {:else if devices.length === 0}
        <div class="sidebar-state"><p>No block devices found.</p></div>
      {:else}
        {#each devices as device (device.id)}
          <button class:selected={device.id === selectedId} class="device-card" on:click={() => chooseDevice(device)}>
            <span class="drive-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M5 5h14v14H5zM8 15h.01M11 15h5"/></svg></span>
            <span class="device-copy">
              <span class="device-name">{device.displayName}</span>
              <span class="device-meta">{formatBytes(device.capacityBytes)} · {device.transport || 'unknown bus'}</span>
            </span>
            <span class:ok={device.likelyExternal && !device.systemDisk} class:locked={device.systemDisk} class="status-dot"></span>
          </button>
        {/each}
      {/if}
    </div>

    <footer class="sidebar-footer"><span class="safe-dot"></span>Raw block writes are disabled</footer>
  </aside>

  <main class="workspace">
    {#if selected}
      <section class="device-hero">
        <div>
          <div class="eyebrow">Selected device</div>
          <h1>{selected.displayName}</h1>
          <div class="hero-meta">
            <span>{selected.path}</span><span>•</span><span>{formatBytes(selected.capacityBytes)}</span>
            {#if selected.transport}<span>•</span><span>{selected.transport.toUpperCase()}</span>{/if}
          </div>
        </div>
        <div class:danger={selected.systemDisk || selected.readOnly} class:good={selected.rawTest.allowed} class="safety-pill">
          <span></span>{safetyLabel(selected)}
        </div>
      </section>

      <section class="stat-grid">
        <article><span>Vendor</span><strong>{selected.vendor || 'Unknown'}</strong></article>
        <article><span>Model</span><strong>{selected.model || 'Unknown'}</strong></article>
        <article><span>Filesystem</span><strong>{selected.fileSystems.join(', ') || 'Unknown'}</strong></article>
        <article><span>Mounts</span><strong>{selected.mountPoints.length}</strong></article>
      </section>

      <section class="panel verification-panel">
        <div class="panel-title-row">
          <div><div class="eyebrow">Non-destructive test</div><h2>Filesystem verification</h2></div>
          <span class="mode-badge">Temporary files only</span>
        </div>

        {#if canVerifyFilesystem(selected)}
          <div class="controls-grid">
            <label>
              <span>Mount point</span>
              <select bind:value={selectedMount} disabled={runState === 'running' || runState === 'stopping'}>
                {#each selected.mountPoints as mount}<option value={mount}>{mount}</option>{/each}
              </select>
            </label>
            <label>
              <span>Test size</span>
              <select bind:value={testBytes} disabled={runState === 'running' || runState === 'stopping'}>
                {#each testSizes as size}<option value={size.value}>{size.label} · {size.detail}</option>{/each}
              </select>
            </label>
          </div>

          <div class="map-frame">
            <div class="map-header">
              <div><strong>{runState === 'running' ? (progress?.phase === 'verify' ? 'Verifying data' : 'Writing test data') : runState === 'stopping' ? 'Stopping safely' : runState === 'success' ? 'Verification complete' : 'Region map'}</strong><span>{regions.length ? `${regions.length} regions` : 'Starts when the test runs'}</span></div>
              <div class="progress-number">{progressPercent}%</div>
            </div>
            <div class="progress-track"><span style:width={`${progressPercent}%`}></span></div>
            {#if regions.length > 0}
              <div class="region-grid" aria-label="Verification region map">
                {#each regions as region}
                  <span class={`region ${region.state}`} title={`Region ${region.index + 1}: ${region.state}`}></span>
                {/each}
              </div>
            {:else}
              <div class="map-placeholder">
                <svg viewBox="0 0 48 48"><path d="M9 11h30v26H9zM14 17h20M14 23h20M14 29h12"/></svg>
                <span>No verification data yet</span>
              </div>
            {/if}
            <div class="legend"><span><i class="pending"></i>Pending</span><span><i class="writing"></i>Written</span><span><i class="valid"></i>Verified</span><span><i class="corrupt"></i>Error</span></div>
          </div>

          {#if runError}<div class="alert error"><strong>Verification failed</strong><span>{runError}</span></div>{/if}
          {#if runState === 'success' && report}<div class="alert success"><strong>Verification completed</strong><span>{formatBytes(report.bytesVerified)} read back successfully in {elapsedSeconds}s.</span></div>{/if}

          <div class="action-row">
            <p>This check writes and removes temporary files on <strong>{selectedMount}</strong>. It never opens the raw block device.</p>
            {#if runState === 'running'}
              <button class="secondary danger-button" on:click={stopVerification}>Stop test</button>
            {:else if runState === 'stopping'}
              <button class="secondary" disabled><span class="spinner small"></span>Stopping…</button>
            {:else}
              <button class="primary" on:click={runVerification} disabled={!verifyEnabled}>Start verification</button>
            {/if}
          </div>
        {:else}
          <div class="protected-state">
            <div class="shield-icon"><svg viewBox="0 0 32 32"><path d="M16 4 26 8v7c0 6.5-4 10.5-10 13-6-2.5-10-6.5-10-13V8l10-4Z"/><path d="m12 16 2.5 2.5L20 13"/></svg></div>
            <div><strong>Filesystem verification unavailable</strong><p>Select a mounted, writable external device. StorVerity blocks internal, system and read-only storage.</p></div>
          </div>
        {/if}
      </section>

      <section class="raw-banner">
        <div><span class="lock-icon">◆</span><div><strong>Raw capacity probe</strong><p>Direct block-device testing remains locked until Phase 4 safety gates are complete.</p></div></div>
        <span class="coming-soon">Planned</span>
      </section>
    {:else if !loading}
      <div class="empty-workspace"><h1>No device selected</h1><p>Connect a removable drive and refresh the device list.</p></div>
    {/if}
  </main>
</div>
