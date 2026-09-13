export type Severity = 'deny' | 'warning';
export type RegionState = 'pending' | 'writing' | 'valid' | 'corrupt' | 'read-error' | 'write-error';

export interface SafetyReason { code: string; severity: Severity; message: string }
export interface RawTestDecision { allowed: boolean; reasons?: SafetyReason[] }
export interface DeviceCard {
  id: string; path: string; displayName: string; vendor?: string; model?: string; serial?: string;
  transport?: string; capacityBytes: number; mountPoints: string[]; fileSystems: string[];
  readOnly: boolean; systemDisk: boolean; likelyExternal: boolean; rawTest: RawTestDecision;
}
export interface RegionCell { index: number; state: RegionState }

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
  if (device.rawTest.allowed) return 'Eligible for raw test';
  if (device.rawTest.reasons?.some((reason) => reason.code === 'mounted')) return 'Mounted';
  return 'Protected';
}

export function createRegionCells(total: number): RegionCell[] {
  if (!Number.isInteger(total) || total < 0) throw new Error('region total must be a non-negative integer');
  return Array.from({ length: total }, (_, index) => ({ index, state: 'pending' as const }));
}

export function updateRegion(cells: RegionCell[], index: number, state: RegionState): RegionCell[] {
  if (index < 0 || index >= cells.length) return cells;
  return cells.map((cell) => cell.index === index ? { ...cell, state } : cell);
}
