import { ref } from 'vue'

export type Theme = 'dark'

const theme = ref<Theme>('dark')

export function useTheme() {
  return { theme }
}
