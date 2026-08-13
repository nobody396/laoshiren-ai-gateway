type AudioContextWithWebkit = typeof window & {
  webkitAudioContext?: typeof AudioContext
}

const PRINT_DURATION_MS = 7200
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
  const completionBellAt = startAt + feedDuration + cutterStartDelay + 0.22
  const completionBellDuration = 0.92
  const stopAt = completionBellAt + completionBellDuration
  const sources: AudioScheduledSourceNode[] = []

  const master = context.createGain()
  master.gain.setValueAtTime(0.68, startAt)
  master.gain.setValueAtTime(0.68, startAt + feedDuration)
  master.gain.exponentialRampToValueAtTime(0.0001, stopAt)
  master.connect(context.destination)

  const completionCompressor = context.createDynamicsCompressor()
  const completionGain = context.createGain()
  completionCompressor.threshold.value = -12
  completionCompressor.knee.value = 8
  completionCompressor.ratio.value = 4
  completionCompressor.attack.value = 0.002
  completionCompressor.release.value = 0.18
  completionGain.gain.setValueAtTime(0.92, completionBellAt)
  completionGain.gain.exponentialRampToValueAtTime(0.0001, stopAt)
  completionCompressor.connect(completionGain)
  completionGain.connect(context.destination)

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
  noiseHighPass.frequency.value = 560
  noiseLowPass.type = 'lowpass'
  noiseLowPass.frequency.value = 4200
  noiseGain.gain.setValueAtTime(0.018, startAt)

  // Tight paper slaps imitate a banknote counter's rubber wheels rather than
  // a domestic thermal printer's slower dotted chatter.
  for (let offset = 0; offset < feedDuration - 0.05; offset += 0.046) {
    const pulseAt = startAt + offset
    noiseGain.gain.setValueAtTime(0.014, pulseAt)
    noiseGain.gain.linearRampToValueAtTime(0.078, pulseAt + 0.003)
    noiseGain.gain.exponentialRampToValueAtTime(0.014, pulseAt + 0.021)
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
  motor.frequency.setValueAtTime(92, startAt)
  motor.frequency.linearRampToValueAtTime(116, startAt + 0.28)
  motor.frequency.setValueAtTime(116, startAt + feedDuration - 0.2)
  motor.frequency.exponentialRampToValueAtTime(62, startAt + feedDuration)
  motorGain.gain.setValueAtTime(0.022, startAt)
  motorGain.gain.exponentialRampToValueAtTime(0.0001, startAt + feedDuration)
  motor.connect(motorGain)
  motorGain.connect(master)
  motor.start(startAt)
  motor.stop(startAt + feedDuration)
  sources.push(motor)

  const headWhine = context.createOscillator()
  const headGain = context.createGain()
  headWhine.type = 'square'
  headWhine.frequency.setValueAtTime(410, startAt)
  headWhine.frequency.linearRampToValueAtTime(470, startAt + 0.28)
  headWhine.frequency.setValueAtTime(470, startAt + feedDuration)
  headGain.gain.setValueAtTime(0.0028, startAt)
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

  // Cash-counter finish: an abrupt tray clack, followed by a short bright
  // metal strike. This reads as “counting complete” instead of an oven timer.
  const trayClack = context.createOscillator()
  const trayClackGain = context.createGain()
  trayClack.type = 'triangle'
  trayClack.frequency.setValueAtTime(190, completionBellAt)
  trayClack.frequency.exponentialRampToValueAtTime(48, completionBellAt + 0.085)
  trayClackGain.gain.setValueAtTime(0.34, completionBellAt)
  trayClackGain.gain.exponentialRampToValueAtTime(0.0001, completionBellAt + 0.1)
  trayClack.connect(trayClackGain)
  trayClackGain.connect(completionCompressor)
  trayClack.start(completionBellAt)
  trayClack.stop(completionBellAt + 0.11)
  sources.push(trayClack)

  const metalStrikeAt = completionBellAt + 0.055
  for (const [frequency, gainValue, duration] of [
    [1320, 0.3, completionBellDuration],
    [1980, 0.14, 0.62],
    [2640, 0.065, 0.38]
  ] as const) {
    const bell = context.createOscillator()
    const bellGain = context.createGain()
    const bellFilter = context.createBiquadFilter()
    bell.type = 'sine'
    bell.frequency.setValueAtTime(frequency, metalStrikeAt)
    bell.frequency.exponentialRampToValueAtTime(frequency * 0.982, metalStrikeAt + duration)
    bellGain.gain.setValueAtTime(0.0001, metalStrikeAt)
    bellGain.gain.linearRampToValueAtTime(gainValue, metalStrikeAt + 0.004)
    bellGain.gain.exponentialRampToValueAtTime(0.0001, metalStrikeAt + duration)
    bellFilter.type = 'highpass'
    bellFilter.frequency.value = 760
    bell.connect(bellFilter)
    bellFilter.connect(bellGain)
    bellGain.connect(completionCompressor)
    bell.start(metalStrikeAt)
    bell.stop(metalStrikeAt + duration + 0.02)
    sources.push(bell)
  }

  void context.resume().catch(() => undefined)

  let stopped = false
  const timeoutId = window.setTimeout(
    () => stop(),
    PRINT_DURATION_MS + CUTTER_START_DELAY_MS + (completionBellDuration * 1000) + 500 + SOUND_CLEANUP_GRACE_MS
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
