import { joinOffice, leaveOffice, listMembers, updateStatus, Member, MemberStatus } from './api';
import { P2PCall } from './p2p';

// ─── State ───────────────────────────────────────────────────────────────────
let myMember: Member | null = null;
let members: Map<string, Member> = new Map();
let ws: WebSocket | null = null;
let activeCall: P2PCall | null = null;

// ─── DOM refs ─────────────────────────────────────────────────────────────────
const joinForm     = document.getElementById('join-form')     as HTMLFormElement;
const inputName    = document.getElementById('input-name')    as HTMLInputElement;
const inputPlat    = document.getElementById('input-platform') as HTMLSelectElement;
const btnJoin      = document.getElementById('btn-join')      as HTMLButtonElement;
const btnLeave     = document.getElementById('btn-leave')     as HTMLButtonElement;
const statusRow    = document.getElementById('status-row')    as HTMLDivElement;
const myStatusSel  = document.getElementById('my-status')     as HTMLSelectElement;
const membersList  = document.getElementById('members-list')  as HTMLDivElement;
const memberCount  = document.getElementById('member-count')  as HTMLSpanElement;
const callArea     = document.getElementById('call-area')     as HTMLDivElement;
const localVideo   = document.getElementById('local-video')   as HTMLVideoElement;
const remoteVideo  = document.getElementById('remote-video')  as HTMLVideoElement;
const btnEndCall   = document.getElementById('btn-end-call')  as HTMLButtonElement;
const toast        = document.getElementById('toast')         as HTMLDivElement;

// ─── Toast helper ─────────────────────────────────────────────────────────────
let toastTimer = 0;
function showToast(msg: string): void {
  toast.textContent = msg;
  toast.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => toast.classList.remove('show'), 3000);
}

// ─── Render members ───────────────────────────────────────────────────────────
function renderMembers(): void {
  memberCount.textContent = String(members.size);
  membersList.innerHTML = '';
  for (const m of members.values()) {
    const isMe = m.id === myMember?.id;
    const card = document.createElement('div');
    card.className = 'member-card';
    card.innerHTML = `
      <div class="name">${esc(m.name)}${isMe ? ' (you)' : ''}</div>
      <div class="platform">${esc(m.platform)}</div>
      <span class="status-badge status-${m.status}">${m.status}</span>
      ${!isMe && myMember ? `<button class="btn-call" data-peer="${m.id}">📞 Call</button>` : ''}
    `;
    if (!isMe && myMember) {
      card.querySelector<HTMLButtonElement>('.btn-call')!.addEventListener('click', () => startCall(m.id));
    }
    membersList.appendChild(card);
  }
}

function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ─── WebSocket ────────────────────────────────────────────────────────────────
interface WsEvent {
  type: string;
  payload: unknown;
}

function connectWS(): void {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws`);

  ws.onmessage = async (evt) => {
    const event: WsEvent = JSON.parse(evt.data);
    switch (event.type) {
      case 'member_joined': {
        const m = event.payload as Member;
        members.set(m.id, m);
        renderMembers();
        if (m.id !== myMember?.id) showToast(`${m.name} joined the office`);
        break;
      }
      case 'member_left': {
        const { id } = event.payload as { id: string };
        const leaving = members.get(id);
        members.delete(id);
        renderMembers();
        if (leaving) showToast(`${leaving.name} left the office`);
        break;
      }
      case 'status_changed': {
        const m = event.payload as Member;
        members.set(m.id, m);
        renderMembers();
        break;
      }
      case 'signal': {
        await handleSignal(event.payload as SignalPayload);
        break;
      }
    }
  };

  ws.onclose = () => {
    if (myMember) setTimeout(connectWS, 2000); // auto-reconnect while in office
  };
}

// ─── WebRTC signaling ─────────────────────────────────────────────────────────
interface SignalPayload {
  from: string;
  to: string;
  signal: {
    type: 'offer' | 'answer' | 'candidate';
    sdp?: RTCSessionDescriptionInit;
    candidate?: RTCIceCandidateInit;
  };
}

async function handleSignal(payload: SignalPayload): Promise<void> {
  if (!myMember || payload.to !== myMember.id) return;
  const { from, signal } = payload;

  if (signal.type === 'offer' && signal.sdp) {
    showToast(`Incoming call from ${members.get(from)?.name ?? from}…`);
    if (activeCall) activeCall.hangUp();
    activeCall = new P2PCall(myMember.id, from, onRemoteStream);
    const localStream = await activeCall.answerCall(signal.sdp);
    showCallUI(localStream);
  } else if (signal.type === 'answer' && signal.sdp && activeCall) {
    await activeCall.handleAnswer(signal.sdp);
  } else if (signal.type === 'candidate' && signal.candidate && activeCall) {
    await activeCall.handleCandidate(signal.candidate);
  }
}

function onRemoteStream(stream: MediaStream): void {
  remoteVideo.srcObject = stream;
}

// ─── Call helpers ─────────────────────────────────────────────────────────────
async function startCall(peerId: string): Promise<void> {
  if (!myMember) return;
  if (activeCall) activeCall.hangUp();
  activeCall = new P2PCall(myMember.id, peerId, onRemoteStream);
  try {
    const localStream = await activeCall.startCall();
    showCallUI(localStream);
    showToast(`Calling ${members.get(peerId)?.name ?? peerId}…`);
  } catch (err) {
    showToast('Could not access camera/microphone');
    console.error(err);
    activeCall = null;
  }
}

function showCallUI(localStream: MediaStream): void {
  localVideo.srcObject = localStream;
  callArea.style.display = 'block';
}

function endCall(): void {
  activeCall?.hangUp();
  activeCall = null;
  localVideo.srcObject = null;
  remoteVideo.srcObject = null;
  callArea.style.display = 'none';
}

// ─── Join / Leave ─────────────────────────────────────────────────────────────
joinForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const name = inputName.value.trim();
  if (!name) return;
  try {
    myMember = await joinOffice(name, inputPlat.value);
    const all = await listMembers();
    members.clear();
    for (const m of all) members.set(m.id, m);
    renderMembers();
    connectWS();
    btnJoin.style.display = 'none';
    btnLeave.style.display = '';
    statusRow.style.display = 'flex';
    inputName.disabled = true;
    inputPlat.disabled = true;
    showToast('You joined the virtual office!');
  } catch (err) {
    showToast('Failed to join. Is the server running?');
    console.error(err);
  }
});

btnLeave.addEventListener('click', async () => {
  if (!myMember) return;
  try {
    await leaveOffice(myMember.id);
  } finally {
    endCall();
    ws?.close();
    ws = null;
    myMember = null;
    members.clear();
    renderMembers();
    btnJoin.style.display = '';
    btnLeave.style.display = 'none';
    statusRow.style.display = 'none';
    inputName.disabled = false;
    inputPlat.disabled = false;
    showToast('You left the virtual office.');
  }
});

myStatusSel.addEventListener('change', async () => {
  if (!myMember) return;
  try {
    const updated = await updateStatus(myMember.id, myStatusSel.value as MemberStatus);
    myMember = updated;
    members.set(updated.id, updated);
    renderMembers();
  } catch (err) {
    console.error(err);
  }
});

btnEndCall.addEventListener('click', endCall);

// ─── Initial member list load ─────────────────────────────────────────────────
(async () => {
  try {
    const all = await listMembers();
    members.clear();
    for (const m of all) members.set(m.id, m);
    renderMembers();
  } catch {
    // server may not be reachable yet
  }
})();
