import { sendSignal } from './api';

const ICE_SERVERS: RTCIceServer[] = [
  { urls: 'stun:stun.l.google.com:19302' },
];

export type OnRemoteStreamCallback = (stream: MediaStream) => void;

export class P2PCall {
  private pc: RTCPeerConnection;
  private localStream: MediaStream | null = null;
  private myId: string;
  private peerId: string;
  private onRemoteStream: OnRemoteStreamCallback;

  constructor(myId: string, peerId: string, onRemoteStream: OnRemoteStreamCallback) {
    this.myId = myId;
    this.peerId = peerId;
    this.onRemoteStream = onRemoteStream;
    this.pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });

    this.pc.onicecandidate = (e) => {
      if (e.candidate) {
        sendSignal(this.myId, this.peerId, { type: 'candidate', candidate: e.candidate }).catch(console.error);
      }
    };

    this.pc.ontrack = (e) => {
      if (e.streams[0]) this.onRemoteStream(e.streams[0]);
    };
  }

  /** Start a call: acquire local media, create an offer and send it. */
  async startCall(): Promise<MediaStream> {
    this.localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
    for (const track of this.localStream.getTracks()) {
      this.pc.addTrack(track, this.localStream);
    }
    const offer = await this.pc.createOffer();
    await this.pc.setLocalDescription(offer);
    await sendSignal(this.myId, this.peerId, { type: 'offer', sdp: offer });
    return this.localStream;
  }

  /** Answer an incoming offer. */
  async answerCall(offerSdp: RTCSessionDescriptionInit): Promise<MediaStream> {
    this.localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
    for (const track of this.localStream.getTracks()) {
      this.pc.addTrack(track, this.localStream);
    }
    await this.pc.setRemoteDescription(offerSdp);
    const answer = await this.pc.createAnswer();
    await this.pc.setLocalDescription(answer);
    await sendSignal(this.myId, this.peerId, { type: 'answer', sdp: answer });
    return this.localStream;
  }

  /** Handle an answer from the remote peer. */
  async handleAnswer(answerSdp: RTCSessionDescriptionInit): Promise<void> {
    await this.pc.setRemoteDescription(answerSdp);
  }

  /** Handle an ICE candidate from the remote peer. */
  async handleCandidate(candidate: RTCIceCandidateInit): Promise<void> {
    await this.pc.addIceCandidate(new RTCIceCandidate(candidate));
  }

  /** Hang up the call and stop local tracks. */
  hangUp(): void {
    this.localStream?.getTracks().forEach((t) => t.stop());
    this.pc.close();
  }
}
