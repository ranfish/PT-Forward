import { ref, watchEffect } from 'vue'

export function useTheme() {
  const isDark = ref(localStorage.getItem('pt-forward-theme') === 'dark')

  function toggle() {
    isDark.value = !isDark.value
  }

  watchEffect(() => {
    localStorage.setItem('pt-forward-theme', isDark.value ? 'dark' : 'light')
    document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
    if (isDark.value) {
      document.body.classList.add('dark')
    } else {
      document.body.classList.remove('dark')
    }
  })

  return { isDark, toggle }
}
