import type { DeviceCard, RawProbeOutcome, VerificationOutcome } from './domain';

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

export interface RawProbeChallenge {
  token: string;
  deviceId: string;
  path: string;
  displayName: string;
  capacityBytes: number;
  confirmationText: string;
  expiresAtUnix: number;
}

export interface RawProbeRequest {
  deviceId: string;
  challengeToken: string;
  confirmation: string;
  samples: number;
  blockBytes: number;
}

export interface RawProbeProgress {
  phase: 'snapshot' | 'write' | 'verify' | 'restore';
  sample: number;
  samplesTotal: number;
  offsetBytes: number;
  outcome: RawProbeOutcome;
  error?: string;
}

export interface RawProbeSampleResult {
  index: number;
  offsetBytes: number;
  outcome: RawProbeOutcome;
  error?: string;
  restored: boolean;
  restoreError?: string;
}

export interface RawProbeReport {
  advertisedBytes: number;
  blockBytes: number;
  samples: number;
  validSamples: number;
  corruptSamples: number;
  readErrors: number;
  writeErrors: number;
  restoreErrors: number;
  validatedThroughBytes: number;
  suspectFakeCapacity: boolean;
  restored: boolean;
  results: RawProbeSampleResult[];
}

interface DesktopBinding {
  ListDevices(): Promise<DeviceCard[]>;
  StartVerification(request: VerificationRequest): Promise<VerificationReport>;
  CancelVerification(): Promise<boolean>;
  VerificationActive(): Promise<boolean>;
  PrepareRawProbe(deviceId: string): Promise<RawProbeChallenge>;
  StartRawProbe(request: RawProbeRequest): Promise<RawProbeReport>;
  CancelRawProbe(): Promise<boolean>;
  RawProbeActive(): Promise<boolean>;
  LastReportJSON(): Promise<string>;
  LastReportText(): Promise<string>;
  ExportLastReport(format: string): Promise<string>;
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

export function prepareRawProbe(deviceId: string): Promise<RawProbeChallenge> {
  return desktop().PrepareRawProbe(deviceId);
}

export function startRawProbe(request: RawProbeRequest): Promise<RawProbeReport> {
  return desktop().StartRawProbe(request);
}

export function cancelRawProbe(): Promise<boolean> {
  return desktop().CancelRawProbe();
}

export function rawProbeActive(): Promise<boolean> {
  return desktop().RawProbeActive();
}

export function lastReportJSON(): Promise<string> {
  return desktop().LastReportJSON();
}

export function lastReportText(): Promise<string> {
  return desktop().LastReportText();
}

export function exportLastReport(format: 'text' | 'json'): Promise<string> {
  return desktop().ExportLastReport(format);
}

export function onVerificationProgress(callback: (progress: VerificationProgress) => void): () => void {
  const runtime = window.runtime;
  if (!runtime) return () => undefined;
  return runtime.EventsOn('verification:progress', (payload) => callback(payload as VerificationProgress));
}

export function onRawProbeProgress(callback: (progress: RawProbeProgress) => void): () => void {
  const runtime = window.runtime;
  if (!runtime) return () => undefined;
  return runtime.EventsOn('rawprobe:progress', (payload) => callback(payload as RawProbeProgress));
}
