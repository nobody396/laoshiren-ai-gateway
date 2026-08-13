type AudioContextWithWebkit = typeof window & {
  webkitAudioContext?: typeof AudioContext
}

const PRINT_DURATION_MS = 6000
const CUTTER_DURATION_MS = 900
const CUTTER_START_DELAY_MS = 0
const SOUND_CLEANUP_GRACE_MS = 200

export function playThermalPrinterSound(): (() => void) | null {
  if (typeof window === 'undefined') return null

  const AudioContextConstructor = window.AudioContext
    || (window as AudioContextWithWebkit).webkitAudioContext
  if (!AudioContextConstructor) return null

  const context = new AudioContextConstructor()
  const startAt = context.currentTime + 0.02
  const feedDuration = PRINT_DURATION_MS / 1000
  const cutterStartDelay = CUTTER_START_DELAY_MS / 1000
  const cutterDuration = CUTTER_DURATION_MS / 1000
  const stopAt = startAt + feedDuration + cutterStartDelay + cutterDuration
  const sources: AudioScheduledSourceNode[] = []

  const master = context.createGain()
  master.gain.setValueAtTime(0.68, startAt)
  master.gain.setValueAtTime(0.68, startAt + feedDuration)
  master.gain.exponentialRampToValueAtTime(0.0001, stopAt)
  master.connect(context.destination)

  const sampleCount = Math.ceil(context.sampleRate * feedDuration)
  const noiseBuffer = context.createBuffer(1, sampleCount, context.sampleRate)
  const noiseData = noiseBuffer.getChannelData(0)
  let previousSample = 0
  for (let index = 0; index < noiseData.length; index += 1) {
    const white = (Math.random() * 2) - 1
    previousSample = (previousSample * 0.84) + (white * 0.16)
    noiseData[index] = previousSample
  }

  const noise = context.createBufferSource()
  const noiseHighPass = context.createBiquadFilter()
  const noiseLowPass = context.createBiquadFilter()
  const noiseGain = context.createGain()
  noise.buffer = noiseBuffer
  noiseHighPass.type = 'highpass'
  noiseHighPass.frequency.value = 430
  noiseLowPass.type = 'lowpass'
  noiseLowPass.frequency.value = 3200
  noiseGain.gain.setValueAtTime(0.024, startAt)

  for (let offset = 0; offset < feedDuration - 0.08; offset += 0.082) {
    const pulseAt = startAt + offset
    noiseGain.gain.setValueAtTime(0.018, pulseAt)
    noiseGain.gain.linearRampToValueAtTime(0.068, pulseAt + 0.006)
    noiseGain.gain.exponentialRampToValueAtTime(0.02, pulseAt + 0.032)
  }

  noise.connect(noiseHighPass)
  noiseHighPass.connect(noiseLowPass)
  noiseLowPass.connect(noiseGain)
  noiseGain.connect(master)
  noise.start(startAt)
  noise.stop(startAt + feedDuration)
  sources.push(noise)

  const motor = context.createOscillator()
  const motorGain = context.createGain()
  motor.type = 'sawtooth'
  motor.frequency.setValueAtTime(74, startAt)
  motor.frequency.linearRampToValueAtTime(82, startAt + feedDuration)
  motorGain.gain.setValueAtTime(0.018, startAt)
  motorGain.gain.exponentialRampToValueAtTime(0.0001, startAt + feedDuration)
  motor.connect(motorGain)
  motorGain.connect(master)
  motor.start(startAt)
  motor.stop(startAt + feedDuration)
  sources.push(motor)

  const headWhine = context.createOscillator()
  const headGain = context.createGain()
  headWhine.type = 'square'
  headWhine.frequency.setValueAtTime(286, startAt)
  headWhine.frequency.linearRampToValueAtTime(338, startAt + feedDuration)
  headGain.gain.setValueAtTime(0.0035, startAt)
  headGain.gain.exponentialRampToValueAtTime(0.0001, startAt + feedDuration)
  headWhine.connect(headGain)
  headGain.connect(master)
  headWhine.start(startAt)
  headWhine.stop(startAt + feedDuration)
  sources.push(headWhine)

  const buttonClick = context.createOscillator()
  const buttonClickGain = context.createGain()
  buttonClick.type = 'triangle'
  buttonClick.frequency.setValueAtTime(176, startAt)
  buttonClick.frequency.exponentialRampToValueAtTime(72, startAt + 0.045)
  buttonClickGain.gain.setValueAtTime(0.095, startAt)
  buttonClickGain.gain.exponentialRampToValueAtTime(0.0001, startAt + 0.06)
  buttonClick.connect(buttonClickGain)
  buttonClickGain.connect(master)
  buttonClick.start(startAt)
  buttonClick.stop(startAt + 0.065)
  sources.push(buttonClick)

  const cutterAt = startAt + feedDuration + cutterStartDelay
  for (const [offset, startFrequency] of [[0, 138], [0.16, 104]] as const) {
    const cutter = context.createOscillator()
    const cutterGain = context.createGain()
    const hitAt = cutterAt + offset
    cutter.type = 'triangle'
    cutter.frequency.setValueAtTime(startFrequency, hitAt)
    cutter.frequency.exponentialRampToValueAtTime(42, hitAt + 0.055)
    cutterGain.gain.setValueAtTime(0.12, hitAt)
    cutterGain.gain.exponentialRampToValueAtTime(0.0001, hitAt + 0.075)
    cutter.connect(cutterGain)
    cutterGain.connect(master)
    cutter.start(hitAt)
    cutter.stop(hitAt + 0.08)
    sources.push(cutter)
  }

  void context.resume().catch(() => undefined)

  let stopped = false
  const timeoutId = window.setTimeout(
    () => stop(),
    PRINT_DURATION_MS + CUTTER_START_DELAY_MS + CUTTER_DURATION_MS + SOUND_CLEANUP_GRACE_MS
  )

  function stop(): void {
    if (stopped) return
    stopped = true
    window.clearTimeout(timeoutId)
    for (const source of sources) {
      try {
        source.stop()
      } catch {
        // A source that has already ended needs no further cleanup.
      }
    }
    void context.close().catch(() => undefined)
  }

  return stop
}
