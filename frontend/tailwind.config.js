/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // ====================================================================
        // SINGLE THEME SOURCE: every stop below is `rgb(var(--color-x) / <alpha-value>)`,
        // never a literal hex. The actual color values live ONLY in
        // src/styles/theme.css. To re-theme the app, edit that file — not this one.
        // The `<alpha-value>` placeholder is what lets Tailwind opacity modifiers
        // (e.g. bg-gray-100/50) work against a CSS-variable-backed color.
        // ====================================================================

        // gray — replaces Tailwind's undefined-default cool gray (6,516 usages).
        // Warm papyrus→ink ladder, see theme.css.
        gray: {
          50: 'rgb(var(--color-gray-50) / <alpha-value>)',
          100: 'rgb(var(--color-gray-100) / <alpha-value>)',
          200: 'rgb(var(--color-gray-200) / <alpha-value>)',
          300: 'rgb(var(--color-gray-300) / <alpha-value>)',
          400: 'rgb(var(--color-gray-400) / <alpha-value>)',
          500: 'rgb(var(--color-gray-500) / <alpha-value>)',
          600: 'rgb(var(--color-gray-600) / <alpha-value>)',
          700: 'rgb(var(--color-gray-700) / <alpha-value>)',
          800: 'rgb(var(--color-gray-800) / <alpha-value>)',
          900: 'rgb(var(--color-gray-900) / <alpha-value>)',
          950: 'rgb(var(--color-gray-950) / <alpha-value>)'
        },
        // primary — terracotta ramp (904 usages). Was blue, now the brand action color.
        primary: {
          50: 'rgb(var(--color-primary-50) / <alpha-value>)',
          100: 'rgb(var(--color-primary-100) / <alpha-value>)',
          200: 'rgb(var(--color-primary-200) / <alpha-value>)',
          300: 'rgb(var(--color-primary-300) / <alpha-value>)',
          400: 'rgb(var(--color-primary-400) / <alpha-value>)',
          500: 'rgb(var(--color-primary-500) / <alpha-value>)',
          600: 'rgb(var(--color-primary-600) / <alpha-value>)',
          700: 'rgb(var(--color-primary-700) / <alpha-value>)',
          800: 'rgb(var(--color-primary-800) / <alpha-value>)',
          900: 'rgb(var(--color-primary-900) / <alpha-value>)',
          950: 'rgb(var(--color-primary-950) / <alpha-value>)'
        },
        // red — danger ramp (684 usages).
        red: {
          50: 'rgb(var(--color-red-50) / <alpha-value>)',
          100: 'rgb(var(--color-red-100) / <alpha-value>)',
          200: 'rgb(var(--color-red-200) / <alpha-value>)',
          300: 'rgb(var(--color-red-300) / <alpha-value>)',
          400: 'rgb(var(--color-red-400) / <alpha-value>)',
          500: 'rgb(var(--color-red-500) / <alpha-value>)',
          600: 'rgb(var(--color-red-600) / <alpha-value>)',
          700: 'rgb(var(--color-red-700) / <alpha-value>)',
          800: 'rgb(var(--color-red-800) / <alpha-value>)',
          900: 'rgb(var(--color-red-900) / <alpha-value>)',
          950: 'rgb(var(--color-red-950) / <alpha-value>)'
        },
        // green / emerald — success ramp (662 usages combined), identical values.
        green: {
          50: 'rgb(var(--color-green-50) / <alpha-value>)',
          100: 'rgb(var(--color-green-100) / <alpha-value>)',
          200: 'rgb(var(--color-green-200) / <alpha-value>)',
          300: 'rgb(var(--color-green-300) / <alpha-value>)',
          400: 'rgb(var(--color-green-400) / <alpha-value>)',
          500: 'rgb(var(--color-green-500) / <alpha-value>)',
          600: 'rgb(var(--color-green-600) / <alpha-value>)',
          700: 'rgb(var(--color-green-700) / <alpha-value>)',
          800: 'rgb(var(--color-green-800) / <alpha-value>)',
          900: 'rgb(var(--color-green-900) / <alpha-value>)',
          950: 'rgb(var(--color-green-950) / <alpha-value>)'
        },
        emerald: {
          50: 'rgb(var(--color-emerald-50) / <alpha-value>)',
          100: 'rgb(var(--color-emerald-100) / <alpha-value>)',
          200: 'rgb(var(--color-emerald-200) / <alpha-value>)',
          300: 'rgb(var(--color-emerald-300) / <alpha-value>)',
          400: 'rgb(var(--color-emerald-400) / <alpha-value>)',
          500: 'rgb(var(--color-emerald-500) / <alpha-value>)',
          600: 'rgb(var(--color-emerald-600) / <alpha-value>)',
          700: 'rgb(var(--color-emerald-700) / <alpha-value>)',
          800: 'rgb(var(--color-emerald-800) / <alpha-value>)',
          900: 'rgb(var(--color-emerald-900) / <alpha-value>)',
          950: 'rgb(var(--color-emerald-950) / <alpha-value>)'
        },
        // amber — warning ramp (487 usages).
        amber: {
          50: 'rgb(var(--color-amber-50) / <alpha-value>)',
          100: 'rgb(var(--color-amber-100) / <alpha-value>)',
          200: 'rgb(var(--color-amber-200) / <alpha-value>)',
          300: 'rgb(var(--color-amber-300) / <alpha-value>)',
          400: 'rgb(var(--color-amber-400) / <alpha-value>)',
          500: 'rgb(var(--color-amber-500) / <alpha-value>)',
          600: 'rgb(var(--color-amber-600) / <alpha-value>)',
          700: 'rgb(var(--color-amber-700) / <alpha-value>)',
          800: 'rgb(var(--color-amber-800) / <alpha-value>)',
          900: 'rgb(var(--color-amber-900) / <alpha-value>)',
          950: 'rgb(var(--color-amber-950) / <alpha-value>)'
        },
        // blue / indigo / slate — info ramp (595 usages combined).
        blue: {
          50: 'rgb(var(--color-blue-50) / <alpha-value>)',
          100: 'rgb(var(--color-blue-100) / <alpha-value>)',
          200: 'rgb(var(--color-blue-200) / <alpha-value>)',
          300: 'rgb(var(--color-blue-300) / <alpha-value>)',
          400: 'rgb(var(--color-blue-400) / <alpha-value>)',
          500: 'rgb(var(--color-blue-500) / <alpha-value>)',
          600: 'rgb(var(--color-blue-600) / <alpha-value>)',
          700: 'rgb(var(--color-blue-700) / <alpha-value>)',
          800: 'rgb(var(--color-blue-800) / <alpha-value>)',
          900: 'rgb(var(--color-blue-900) / <alpha-value>)',
          950: 'rgb(var(--color-blue-950) / <alpha-value>)'
        },
        indigo: {
          50: 'rgb(var(--color-indigo-50) / <alpha-value>)',
          100: 'rgb(var(--color-indigo-100) / <alpha-value>)',
          200: 'rgb(var(--color-indigo-200) / <alpha-value>)',
          300: 'rgb(var(--color-indigo-300) / <alpha-value>)',
          400: 'rgb(var(--color-indigo-400) / <alpha-value>)',
          500: 'rgb(var(--color-indigo-500) / <alpha-value>)',
          600: 'rgb(var(--color-indigo-600) / <alpha-value>)',
          700: 'rgb(var(--color-indigo-700) / <alpha-value>)',
          800: 'rgb(var(--color-indigo-800) / <alpha-value>)',
          900: 'rgb(var(--color-indigo-900) / <alpha-value>)',
          950: 'rgb(var(--color-indigo-950) / <alpha-value>)'
        },
        slate: {
          50: 'rgb(var(--color-slate-50) / <alpha-value>)',
          100: 'rgb(var(--color-slate-100) / <alpha-value>)',
          200: 'rgb(var(--color-slate-200) / <alpha-value>)',
          300: 'rgb(var(--color-slate-300) / <alpha-value>)',
          400: 'rgb(var(--color-slate-400) / <alpha-value>)',
          500: 'rgb(var(--color-slate-500) / <alpha-value>)',
          600: 'rgb(var(--color-slate-600) / <alpha-value>)',
          700: 'rgb(var(--color-slate-700) / <alpha-value>)',
          800: 'rgb(var(--color-slate-800) / <alpha-value>)',
          900: 'rgb(var(--color-slate-900) / <alpha-value>)',
          950: 'rgb(var(--color-slate-950) / <alpha-value>)'
        },
        // warm — was already warm-toned; realigned onto the same papyrus→ink ladder as gray.
        warm: {
          50: 'rgb(var(--color-warm-50) / <alpha-value>)',
          100: 'rgb(var(--color-warm-100) / <alpha-value>)',
          200: 'rgb(var(--color-warm-200) / <alpha-value>)',
          300: 'rgb(var(--color-warm-300) / <alpha-value>)',
          400: 'rgb(var(--color-warm-400) / <alpha-value>)',
          500: 'rgb(var(--color-warm-500) / <alpha-value>)',
          600: 'rgb(var(--color-warm-600) / <alpha-value>)',
          700: 'rgb(var(--color-warm-700) / <alpha-value>)',
          800: 'rgb(var(--color-warm-800) / <alpha-value>)',
          900: 'rgb(var(--color-warm-900) / <alpha-value>)',
          950: 'rgb(var(--color-warm-950) / <alpha-value>)'
        },
        // accent — legacy secondary-dark palette (0 current usages), kept for compatibility.
        accent: {
          50: 'rgb(var(--color-accent-50) / <alpha-value>)',
          100: 'rgb(var(--color-accent-100) / <alpha-value>)',
          200: 'rgb(var(--color-accent-200) / <alpha-value>)',
          300: 'rgb(var(--color-accent-300) / <alpha-value>)',
          400: 'rgb(var(--color-accent-400) / <alpha-value>)',
          500: 'rgb(var(--color-accent-500) / <alpha-value>)',
          600: 'rgb(var(--color-accent-600) / <alpha-value>)',
          700: 'rgb(var(--color-accent-700) / <alpha-value>)',
          800: 'rgb(var(--color-accent-800) / <alpha-value>)',
          900: 'rgb(var(--color-accent-900) / <alpha-value>)',
          950: 'rgb(var(--color-accent-950) / <alpha-value>)'
        },
        // dark — the palette behind `dark:bg-dark-*` etc (2,124 usages). Was cool
        // slate; now a warm deep-brown-to-black "ink terminal" ladder.
        dark: {
          50: 'rgb(var(--color-dark-50) / <alpha-value>)',
          100: 'rgb(var(--color-dark-100) / <alpha-value>)',
          200: 'rgb(var(--color-dark-200) / <alpha-value>)',
          300: 'rgb(var(--color-dark-300) / <alpha-value>)',
          400: 'rgb(var(--color-dark-400) / <alpha-value>)',
          500: 'rgb(var(--color-dark-500) / <alpha-value>)',
          600: 'rgb(var(--color-dark-600) / <alpha-value>)',
          700: 'rgb(var(--color-dark-700) / <alpha-value>)',
          800: 'rgb(var(--color-dark-800) / <alpha-value>)',
          900: 'rgb(var(--color-dark-900) / <alpha-value>)',
          950: 'rgb(var(--color-dark-950) / <alpha-value>)'
        },

        // Semantic tokens, exposed as Tailwind color names too (e.g. bg-papyrus,
        // text-terracotta) for any new code that wants to reach for the brand
        // vocabulary directly instead of through the numbered ramps.
        papyrus: 'rgb(var(--color-papyrus) / <alpha-value>)',
        marble: 'rgb(var(--color-marble) / <alpha-value>)',
        parchment: 'rgb(var(--color-parchment) / <alpha-value>)',
        stone: 'rgb(var(--color-stone) / <alpha-value>)',
        ink: 'rgb(var(--color-ink) / <alpha-value>)',
        'ink-deep': 'rgb(var(--color-ink-deep) / <alpha-value>)',
        muted: 'rgb(var(--color-muted) / <alpha-value>)',
        terracotta: 'rgb(var(--color-terracotta) / <alpha-value>)',
        'terracotta-dark': 'rgb(var(--color-terracotta-dark) / <alpha-value>)',
        laurel: 'rgb(var(--color-laurel) / <alpha-value>)',
        'laurel-dark': 'rgb(var(--color-laurel-dark) / <alpha-value>)',
        danger: 'rgb(var(--color-danger) / <alpha-value>)',
        success: 'rgb(var(--color-success) / <alpha-value>)',
        warning: 'rgb(var(--color-warning) / <alpha-value>)',
        info: 'rgb(var(--color-info) / <alpha-value>)',
        vellum: 'rgb(var(--color-vellum) / <alpha-value>)'
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        display: [
          'Source Serif 4',
          'Noto Serif SC',
          'Georgia',
          'Times New Roman',
          'serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(0, 0, 0, 0.08)',
        'glass-sm': '0 4px 16px rgba(0, 0, 0, 0.06)',
        glow: '0 0 20px rgb(var(--color-terracotta) / 0.25)',
        'glow-lg': '0 0 40px rgb(var(--color-terracotta) / 0.35)',
        card: '0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06)',
        'card-hover': '0 4px 20px rgba(0, 0, 0, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)',
        // claude.com 风格 box-shadow 边框
        'btn-border': '0 0 0 0.0625rem var(--tw-shadow-color, rgba(0,0,0,0.12))',
        'btn-border-hover': '0 0 0 0.125rem var(--tw-shadow-color, rgba(0,0,0,0.2))'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, rgb(var(--color-primary-500)) 0%, rgb(var(--color-primary-600)) 100%)',
        'gradient-dark': 'linear-gradient(135deg, rgb(var(--color-dark-700)) 0%, rgb(var(--color-dark-950)) 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 40% 20%, rgb(var(--color-terracotta) / 0.12) 0px, transparent 50%), radial-gradient(at 80% 0%, rgb(var(--color-terracotta) / 0.08) 0px, transparent 50%), radial-gradient(at 0% 50%, rgb(var(--color-laurel) / 0.08) 0px, transparent 50%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgb(var(--color-terracotta) / 0.25)' },
          '100%': { boxShadow: '0 0 30px rgb(var(--color-terracotta) / 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
