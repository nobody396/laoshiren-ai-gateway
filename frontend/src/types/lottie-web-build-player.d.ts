declare module 'lottie-web/build/player/lottie_light_canvas' {
  import type { AnimationItem, AnimationConfigWithData } from 'lottie-web'

  interface LottiePlayer {
    loadAnimation(config: AnimationConfigWithData<'canvas'>): AnimationItem
    destroy(name?: string): void
  }

  const lottie: LottiePlayer
  export default lottie
}
