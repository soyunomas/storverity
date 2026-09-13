import test from 'node:test';
import assert from 'node:assert/strict';
import {
  canVerifyFilesystem, createRegionCells, formatBytes, overallProgress, regionStateForProgress,
  safetyLabel, updateRegion, type DeviceCard,
} from '../src/lib/domain.ts';

const base: DeviceCard = {
  id: 'serial:1', path: '/dev/sdb', displayName: 'USB', capacityBytes: 64 * 1024 ** 3,
  mountPoints: [], fileSystems: [], readOnly: false, systemDisk: false,
  likelyExternal: true, rawTest: { allowed: true },
};

test('formatBytes formats binary capacities', () => {
  assert.equal(formatBytes(64 * 1024 ** 3), '64.0 GiB');
  assert.equal(formatBytes(-1), 'Unknown');
});

test('safetyLabel prioritises critical state', () => {
  assert.equal(safetyLabel({ ...base, systemDisk: true }), 'System disk');
  assert.equal(safetyLabel({ ...base, readOnly: true }), 'Read-only');
  assert.equal(safetyLabel(base), 'Raw test eligible');
  assert.equal(safetyLabel({ ...base, rawTest: { allowed: false, reasons: [{ code: 'mounted', severity: 'deny', message: 'Mounted' }] } }), 'Mounted');
});

test('filesystem verification requires mounted external writable media', () => {
  assert.equal(canVerifyFilesystem({ ...base, mountPoints: ['/media/USB'] }), true);
  assert.equal(canVerifyFilesystem(base), false);
  assert.equal(canVerifyFilesystem({ ...base, mountPoints: ['/'], systemDisk: true }), false);
  assert.equal(canVerifyFilesystem({ ...base, mountPoints: ['/media/USB'], likelyExternal: false }), false);
});

test('region reducer creates immutable state transitions and stores failure detail', () => {
  const cells = createRegionCells(3);
  const next = updateRegion(cells, 1, 'read-error', 'short read');
  assert.equal(cells[1].state, 'pending');
  assert.deepEqual(next[1], { index: 1, state: 'read-error', message: 'short read' });
  assert.strictEqual(updateRegion(next, 9, 'corrupt'), next);
});

test('typed verification outcomes map to region states', () => {
  assert.equal(regionStateForProgress({ phase: 'write', outcome: 'written' }), 'writing');
  assert.equal(regionStateForProgress({ phase: 'verify', outcome: 'verified' }), 'valid');
  assert.equal(regionStateForProgress({ phase: 'verify', outcome: 'corrupt' }), 'corrupt');
  assert.equal(regionStateForProgress({ phase: 'verify', outcome: 'read-error' }), 'read-error');
  assert.equal(regionStateForProgress({ phase: 'write', outcome: 'write-error' }), 'write-error');
});

test('overall progress combines write and verify phases', () => {
  assert.equal(overallProgress(undefined), 0);
  assert.equal(overallProgress({ phase: 'write', bytesCompleted: 50, bytesTotal: 100 }), 0.25);
  assert.equal(overallProgress({ phase: 'verify', bytesCompleted: 50, bytesTotal: 100 }), 0.75);
  assert.equal(overallProgress({ phase: 'verify', bytesCompleted: 100, bytesTotal: 100 }), 1);
});
