import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { transcribeASR } from '../api/client'
import { loadASRSettings } from '../settings/asr'

interface UseSpeechToTextOptions {
  onTranscript: (text: string) => void
}

interface ASRTranscribeResponse {
  text?: string
}

function supportsSpeechCapture(): boolean {
  return typeof window !== 'undefined' && Boolean(navigator.mediaDevices?.getUserMedia) && Boolean(window.MediaRecorder)
}

function chooseMimeType(): string | undefined {
  const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']
  for (const candidate of candidates) {
    if (window.MediaRecorder?.isTypeSupported(candidate)) {
      return candidate
    }
  }
  return undefined
}

async function blobToPCM16kMono(blob: Blob): Promise<Blob> {
  const audioContext = new AudioContext()
  try {
    const source = await blob.arrayBuffer()
    const decoded = await audioContext.decodeAudioData(source)
    const pcm = convertBufferToPCM16k(decoded)
    return new Blob([pcm], { type: 'application/octet-stream' })
  } finally {
    await audioContext.close()
  }
}

function convertBufferToPCM16k(buffer: AudioBuffer): ArrayBuffer {
  const sourceRate = buffer.sampleRate
  const targetRate = 16000
  const ratio = sourceRate / targetRate
  const outLength = Math.max(1, Math.round(buffer.length / ratio))
  const output = new Int16Array(outLength)

  for (let i = 0; i < outLength; i += 1) {
    const srcIndex = Math.min(buffer.length - 1, Math.floor(i * ratio))
    let sample = 0
    for (let channel = 0; channel < buffer.numberOfChannels; channel += 1) {
      sample += buffer.getChannelData(channel)[srcIndex]
    }
    sample /= buffer.numberOfChannels
    const clamped = Math.max(-1, Math.min(1, sample))
    output[i] = clamped < 0 ? Math.round(clamped * 32768) : Math.round(clamped * 32767)
  }

  return output.buffer
}

export function useSpeechToText({ onTranscript }: UseSpeechToTextOptions) {
  const supported = useMemo(() => supportsSpeechCapture(), [])
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const mediaStreamRef = useRef<MediaStream | null>(null)
  const audioChunksRef = useRef<Blob[]>([])

  const [isRecording, setIsRecording] = useState(false)
  const [isTranscribing, setIsTranscribing] = useState(false)
  const [error, setError] = useState('')

  const cleanupStream = useCallback(() => {
    mediaStreamRef.current?.getTracks().forEach((track) => track.stop())
    mediaStreamRef.current = null
    mediaRecorderRef.current = null
  }, [])

  const stopRecording = useCallback(() => {
    const recorder = mediaRecorderRef.current
    if (!recorder || recorder.state === 'inactive') {
      return
    }
    recorder.stop()
    setIsRecording(false)
  }, [])

  const startRecording = useCallback(async () => {
    if (!supported) {
      setError('Speech capture is not supported in this browser.')
      return
    }
    if (isRecording || isTranscribing) {
      return
    }

    const settings = loadASRSettings()
    if (!settings.appID || !settings.accessKey) {
      setError('Configure Volcengine ASR in Settings before using the microphone.')
      return
    }

    setError('')
    audioChunksRef.current = []

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      mediaStreamRef.current = stream

      const mimeType = chooseMimeType()
      const recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream)
      mediaRecorderRef.current = recorder

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          audioChunksRef.current.push(event.data)
        }
      }

      recorder.onstop = () => {
        const chunks = audioChunksRef.current
        audioChunksRef.current = []
        cleanupStream()

        if (chunks.length === 0) {
          return
        }

        const audioBlob = new Blob(chunks, { type: recorder.mimeType || 'audio/webm' })
        setIsTranscribing(true)
        void (async () => {
          try {
            const pcmBlob = await blobToPCM16kMono(audioBlob)
            const response = await transcribeASR<ASRTranscribeResponse>({
              appID: settings.appID,
              accessKey: settings.accessKey,
              sampleRate: 16000,
              audio: pcmBlob,
            })
            const text = (response.text ?? '').trim()
            if (text) {
              onTranscript(text)
            }
          } catch (err) {
            setError(err instanceof Error ? err.message : 'Speech transcription failed.')
          } finally {
            setIsTranscribing(false)
          }
        })()
      }

      recorder.start(250)
      setIsRecording(true)
    } catch (err) {
      cleanupStream()
      setError(err instanceof Error ? err.message : 'Unable to start recording.')
    }
  }, [cleanupStream, isRecording, isTranscribing, onTranscript, supported])

  const toggleRecording = useCallback(() => {
    if (isRecording) {
      stopRecording()
      return
    }
    void startRecording()
  }, [isRecording, startRecording, stopRecording])

  useEffect(() => {
    return () => {
      stopRecording()
      cleanupStream()
    }
  }, [cleanupStream, stopRecording])

  return {
    supported,
    isRecording,
    isTranscribing,
    error,
    toggleRecording,
    stopRecording,
    clearError: () => setError(''),
  }
}
