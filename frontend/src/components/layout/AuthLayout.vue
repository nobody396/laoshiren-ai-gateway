<template>
  <div class="auth-shell">
    <div class="auth-shell__texture" aria-hidden="true"></div>

    <div class="auth-shell__container">
      <section class="auth-shell__brand-panel">
        <router-link to="/" class="auth-shell__brand">
          <span class="auth-shell__seal">
            <img :src="siteLogo || '/laoshirenai-icon.jpg'" alt="老实人 AI" />
          </span>
          <span>
            <span class="auth-shell__brand-name">{{ siteName }}</span>
            <span class="auth-shell__brand-tag">A Quiet Place for Code</span>
          </span>
        </router-link>

        <div class="auth-shell__copy">
          <p class="auth-shell__eyebrow">Founded on Craft · MMXXVI</p>
          <h1>进入安静的编码柱廊</h1>
          <p>
            与 Claude、ChatGPT、Gemini 共同思考。每一次调用、每一笔费用，都清晰可查。
          </p>
        </div>

        <blockquote class="auth-shell__quote">
          <p>"The unexamined code is not worth shipping."</p>
          <cite>After Socrates, Apology 38a</cite>
        </blockquote>
      </section>

      <section class="auth-shell__form-panel">
        <div class="auth-shell__form-card">
          <slot />
        </div>

        <div class="auth-shell__footer">
          <slot name="footer" />
        </div>

        <div class="auth-shell__copyright">
          &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || '老实人 AI')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '/laoshirenai-icon.jpg', { allowRelative: true, allowDataUrl: true }))
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  position: relative;
  min-height: 100vh;
  overflow: hidden;
  background:
    radial-gradient(circle at 18% 0%, rgba(242, 233, 210, 0.95) 0%, rgba(242, 233, 210, 0) 54%),
    #f8f3e7;
  color: #1f1a12;
  font-family: 'EB Garamond', 'Noto Serif SC', Georgia, serif;
}

.auth-shell__texture {
  position: fixed;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  opacity: 0.06;
  mix-blend-mode: multiply;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='220' height='220'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 0.12 0 0 0 0 0.1 0 0 0 0 0.07 0 0 0 0.55 0'/></filter><rect width='100%25' height='100%25' filter='url(%23n)'/></svg>");
}

.auth-shell__container {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(360px, 460px);
  gap: 4rem;
  align-items: center;
  width: min(100% - 4rem, 1120px);
  min-height: 100vh;
  margin: 0 auto;
  padding: 4rem 0;
}

.auth-shell__brand-panel {
  display: flex;
  min-height: 36rem;
  flex-direction: column;
  justify-content: space-between;
  border-left: 2px solid #3f5a3a;
  padding: 1.5rem 0 1.5rem 2rem;
}

.auth-shell__brand {
  display: inline-flex;
  align-items: center;
  gap: 0.9rem;
  color: inherit;
  text-decoration: none;
}

.auth-shell__seal {
  width: 3rem;
  height: 3rem;
  overflow: hidden;
  border-radius: 50%;
  background: #9a3b1f;
  box-shadow: inset 0 0 0 2px rgba(250, 246, 236, 0.42), 0 0 0 1px #7a2d17;
}

.auth-shell__seal img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.auth-shell__brand-name,
.auth-shell__brand-tag {
  display: block;
}

.auth-shell__brand-name {
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.18rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  line-height: 1.1;
}

.auth-shell__brand-tag {
  margin-top: 0.2rem;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.64rem;
  font-weight: 600;
  letter-spacing: 0.18em;
  line-height: 1.1;
  text-transform: uppercase;
}

.auth-shell__eyebrow {
  margin: 0 0 1rem;
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.auth-shell__copy h1 {
  max-width: 10ch;
  margin: 0 0 1.25rem;
  color: #13100b;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(3.1rem, 6vw, 5rem);
  font-weight: 500;
  line-height: 1.05;
}

.auth-shell__copy p:last-child {
  max-width: 34rem;
  margin: 0;
  color: #1f1a12;
  font-size: 1.28rem;
  font-style: italic;
  line-height: 1.6;
}

.auth-shell__quote {
  margin: 3rem 0 0;
  padding: 1.25rem 1.5rem;
  background: rgba(239, 230, 207, 0.48);
}

.auth-shell__quote p {
  margin: 0 0 0.75rem;
  color: #1f1a12;
  font-size: 1.2rem;
  font-style: italic;
  font-weight: 600;
  line-height: 1.45;
}

.auth-shell__quote cite {
  color: #8a7d63;
  font-family: 'Inter', sans-serif;
  font-size: 0.65rem;
  font-style: normal;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.auth-shell__form-panel {
  width: 100%;
}

.auth-shell__form-card {
  background: rgba(250, 246, 236, 0.94);
  border: 1px solid rgba(63, 90, 58, 0.2);
  box-shadow: 0 1.5rem 4rem rgba(31, 26, 18, 0.14);
  padding: 2rem;
}

.auth-shell__footer {
  margin-top: 1.5rem;
  text-align: center;
  font-size: 0.98rem;
}

.auth-shell__footer :deep(p) {
  color: #6f634f !important;
}

.auth-shell__footer :deep(a) {
  color: #9a3b1f !important;
}

.auth-shell__copyright {
  margin-top: 2rem;
  color: #8a7d63;
  font-size: 0.82rem;
  text-align: center;
}

.auth-shell__form-card :deep(.text-center > h2) {
  color: #1f1a12 !important;
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-weight: 600;
  letter-spacing: 0;
}

.auth-shell__form-card :deep(.text-center > p) {
  color: #6f634f !important;
  font-family: 'Inter', sans-serif;
}

.auth-shell__form-card :deep(.text-primary-600),
.auth-shell__form-card :deep(.text-primary-500),
.auth-shell__form-card :deep(.text-primary-400),
.auth-shell__form-card :deep(.text-primary-300) {
  color: #9a3b1f !important;
}

.auth-shell__form-card :deep(.border-red-200) {
  border-color: rgba(154, 59, 31, 0.46) !important;
  background: rgba(154, 59, 31, 0.12) !important;
}

.auth-shell__form-card :deep(.bg-red-100) {
  background: rgba(154, 59, 31, 0.16) !important;
}

.auth-shell__form-card :deep(.text-red-800),
.auth-shell__form-card :deep(.text-red-700),
.auth-shell__form-card :deep(.text-red-600),
.auth-shell__form-card :deep(.text-red-500) {
  color: #7a2d17 !important;
}

.auth-shell__form-card :deep(.border-green-200) {
  border-color: rgba(63, 90, 58, 0.42) !important;
  background: rgba(63, 90, 58, 0.12) !important;
}

.auth-shell__form-card :deep(.bg-green-100) {
  background: rgba(63, 90, 58, 0.16) !important;
}

.auth-shell__form-card :deep(.text-green-800),
.auth-shell__form-card :deep(.text-green-700),
.auth-shell__form-card :deep(.text-green-600) {
  color: #3f5a3a !important;
}

.auth-shell :deep(.btn) {
  border-radius: 0;
}

.auth-shell :deep(.btn-primary) {
  background: #9a3b1f;
  box-shadow: none;
}

.auth-shell :deep(.btn-primary:hover) {
  background: #7a2d17;
}

.auth-shell :deep(.btn-secondary) {
  background: #faf6ec;
  border-color: rgba(63, 90, 58, 0.22);
  color: #1f1a12;
  box-shadow: none;
}

.auth-shell :deep(.btn-secondary:hover) {
  background: #f2e9d2;
  border-color: rgba(63, 90, 58, 0.36);
}

.auth-shell :deep(.input) {
  border-radius: 0;
  background: rgba(255, 252, 245, 0.94);
  border-color: rgba(63, 90, 58, 0.22);
  color: #1f1a12;
}

.auth-shell :deep(.input::placeholder) {
  color: #8a7d63;
  opacity: 0.78;
}

.auth-shell :deep(.input:focus) {
  border-color: #9a3b1f;
  --tw-ring-color: rgba(154, 59, 31, 0.24);
}

.auth-shell :deep(.input-label) {
  color: #1f1a12;
  font-family: 'Inter', sans-serif;
  font-size: 0.78rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

@media (max-width: 920px) {
  .auth-shell__container {
    grid-template-columns: 1fr;
    gap: 2rem;
    width: min(100% - 2rem, 540px);
    padding: 2rem 0;
  }

  .auth-shell__brand-panel {
    min-height: auto;
    padding: 0 0 2rem;
    border-left: 0;
    border-bottom: 1px solid rgba(63, 90, 58, 0.18);
  }

  .auth-shell__copy {
    margin-top: 2rem;
  }

  .auth-shell__copy h1 {
    max-width: none;
    font-size: clamp(2.5rem, 14vw, 3.5rem);
  }

  .auth-shell__quote {
    display: none;
  }
}

@media (max-width: 520px) {
  .auth-shell__form-card {
    padding: 1.35rem;
  }
}
</style>
