export type Severity = 'deny' | 'warning';
export type RegionState = 'pending' | 'writing' | 'valid' | 'corrupt' | 'read-error' | 'write-error';
export type VerificationOutcome = 'written' | 'verified' | 'corrupt' | 'read-error' | 'write-error';

export interface SafetyReason { code: string; severity: Severity; message: string }
export interface RawTestDecision { allowed: boolean; reasons?: SafetyReason[] }
export interface DeviceCard {
  id: string; path: string; displayName: string; vendor?: string; model?: string; serial?: string;
  transport?: string; capacityBytes: number; mountPoints: string[]; fileSystems: string[];
  readOnly: boolean; systemDisk: boolean; likelyExternal: boolean; rawTest: RawTestDecision;
}
export interface RegionCell { index: number; state: RegionState; message?: string }
export interface RegionProgressLike { phase: 'write' | 'verify'; outcome: VerificationOutcome; error?: string }
export interface ProgressLike { phase: 'write' | 'verify'; bytesCompleted: number; bytesTotal: number }

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value < 0) return 'Unknown';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  let size = value; let unit = 0;
  while (size >= 1024 && unit < units.length - 1) { size /= 1024; unit += 1; }
  const digits = unit === 0 || size >= 100 ? 0 : size >= 10 ? 1 : 2;
  return `${size.toFixed(digits)} ${units[unit]}`;
}

export function safetyLabel(device: DeviceCard): string {
  if (device.systemDisk) return 'System disk';
  if (device.readOnly) return 'Read-only';
  if (device.rawTest.allowed) return 'Raw test eligible';
  if (device.rawTest.reasons?.some((reason) => reason.code === 'mounted')) return 'Mounted';
  return 'Protected';
}

export function canVerifyFilesystem(device: DeviceCard | undefined): boolean {
  return Boolean(device && device.likelyExternal && !device.systemDisk && !device.readOnly && device.mountPoints.length > 0);
}

export function createRegionCells(total: number): RegionCell[] {
  if (!Number.isInteger(total) || total < 0) throw new Error('region total must be a non-negative integer');
  return Array.from({ length: total }, (_, index) => ({ index, state: 'pending' as const }));
}

export function updateRegion(cells: RegionCell[], index: number, state: RegionState, message?: string): RegionCell[] {
  if (index < 0 || index >= cells.length) return cells;
  return cells.map((cell) => cell.index === index ? { ...cell, state, message } : cell);
}

export function regionStateForProgress(progress: RegionProgressLike): RegionState {
  switch (progress.outcome) {
    case 'written': return 'writing';
    case 'verified': return 'valid';
    case 'corrupt': return 'corrupt';
    case 'read-error': return 'read-error';
    case 'write-error': return 'write-error';
  }
}

export function overallProgress(progress: ProgressLike | undefined): number {
  if (!progress || progress.bytesTotal <= 0) return 0;
  const phaseOffset = progress.phase === 'verify' ? progress.bytesTotal : 0;
  const completed = Math.min(Math.max(progress.bytesCompleted, 0), progress.bytesTotal);
  return Math.min(1, (phaseOffset + completed) / (progress.bytesTotal * 2));
}
