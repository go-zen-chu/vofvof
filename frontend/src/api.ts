export type MemberStatus = 'online' | 'busy' | 'away' | 'offline';

export interface Member {
  id: string;
  name: string;
  platform: string;
  status: MemberStatus;
  lastSeen: string;
}

const BASE = '/api';

/** Register as a member and return the created Member. */
export async function joinOffice(name: string, platform: string): Promise<Member> {
  const res = await fetch(`${BASE}/members`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, platform }),
  });
  if (!res.ok) throw new Error(`join failed: ${res.status}`);
  return res.json();
}

/** Remove member from the office. */
export async function leaveOffice(id: string): Promise<void> {
  const res = await fetch(`${BASE}/members/${id}`, { method: 'DELETE' });
  if (!res.ok && res.status !== 404) throw new Error(`leave failed: ${res.status}`);
}

/** Fetch all current members. */
export async function listMembers(): Promise<Member[]> {
  const res = await fetch(`${BASE}/members`);
  if (!res.ok) throw new Error(`list failed: ${res.status}`);
  return res.json();
}

/** Update this member's status. */
export async function updateStatus(id: string, status: MemberStatus): Promise<Member> {
  const res = await fetch(`${BASE}/members/${id}/status`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  });
  if (!res.ok) throw new Error(`updateStatus failed: ${res.status}`);
  return res.json();
}

/** Send a WebRTC signaling message through the server. */
export async function sendSignal(from: string, to: string, signal: unknown): Promise<void> {
  const res = await fetch(`${BASE}/signal`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ from, to, signal }),
  });
  if (!res.ok) throw new Error(`signal failed: ${res.status}`);
}
