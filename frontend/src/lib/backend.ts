import type { DeviceCard, VerificationOutcome } from './domain';

export interface VerificationRequest {
  deviceId: string;
  mountPoint: string;
  totalBytes: number;
  chunkBytes: number;
}

export interface VerificationProgress {
  phase: 'write' | 'verify';
  region: number;
  regionsTotal: number;
  bytesCompleted: number;
  bytesTotal: number;
  outcome: VerificationOutcome;
  error?: string;
}

export interface VerificationReport {
  bytesWritten: number;
  bytesVerified: number;
  regions: number;
}

interface DesktopBinding {
  ListDevices(): Promise<DeviceCard[]>;
  StartVerification(request: VerificationRequest): Promise<VerificationReport>;
  CancelVerification(): Promise<boolean>;
  VerificationActive(): Promise<boolean>;
}

interface WailsRuntime {
  EventsOn(name: string, callback: (payload: unknown) => void): () => void;
}

declare global {
  interface Window {
    go?: { appservice?: { Desktop?: DesktopBinding } };
    runtime?: WailsRuntime;
  }
}

function desktop(): DesktopBinding {
  const binding = window.go?.appservice?.Desktop;
  if (!binding) throw new Error('StorVerity desktop backend is unavailable');
  return binding;
}

export function listDevices(): Promise<DeviceCard[]> {
  return desktop().ListDevices();
}

export function startVerification(request: VerificationRequest): Promise<VerificationReport> {
  return desktop().StartVerification(request);
}

export function cancelVerification(): Promise<boolean> {
  return desktop().CancelVerification();
}

export function verificationActive(): Promise<boolean> {
  return desktop().VerificationActive();
}

export function onVerificationProgress(callback: (progress: VerificationProgress) => void): () => void {
  const runtime = window.runtime;
  if (!runtime) return () => undefined;
  return runtime.EventsOn('verification:progress', (payload) => callback(payload as VerificationProgress));
}
