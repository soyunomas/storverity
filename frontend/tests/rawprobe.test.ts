import test from 'node:test';
import assert from 'node:assert/strict';
import { createRegionCells, rawRegionStateForProgress, updateRegion } from '../src/lib/domain.ts';

test('raw probe progress maps write/verify failures into region states', () => {
  assert.equal(rawRegionStateForProgress({ phase: 'snapshot', outcome: 'snapshot' }), undefined);
  assert.equal(rawRegionStateForProgress({ phase: 'write', outcome: 'written' }), 'writing');
  assert.equal(rawRegionStateForProgress({ phase: 'verify', outcome: 'valid' }), 'valid');
  assert.equal(rawRegionStateForProgress({ phase: 'verify', outcome: 'corrupt' }), 'corrupt');
  assert.equal(rawRegionStateForProgress({ phase: 'verify', outcome: 'read-error' }), 'read-error');
  assert.equal(rawRegionStateForProgress({ phase: 'write', outcome: 'write-error' }), 'write-error');
  assert.equal(rawRegionStateForProgress({ phase: 'restore', outcome: 'restore-error' }), 'restore-error');
  assert.equal(rawRegionStateForProgress({ phase: 'restore', outcome: 'valid' }), undefined);
});

test('raw restore error can annotate a sampled region', () => {
  const cells = createRegionCells(2);
  const next = updateRegion(cells, 1, 'restore-error', 'restore failed');
  assert.equal(next[1].state, 'restore-error');
  assert.equal(next[1].message, 'restore failed');
});
