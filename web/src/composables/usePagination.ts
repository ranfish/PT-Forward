import { ref } from 'vue'
import type { ApiResponse, PaginatedData } from '@/api/types'

export function usePagination<T = Record<string, unknown>>(
  fetchFn: (page: number, size: number) => Promise<{ data: ApiResponse<PaginatedData<T>> }>,
  defaultSize = 20,
) {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const data = ref<T[]>([])
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(defaultSize)

  async function fetch(page?: number) {
    if (page !== undefined) currentPage.value = page
    loading.value = true
    error.value = null
    try {
      const resp = await fetchFn(currentPage.value, pageSize.value)
      const body = resp.data
      data.value = body.data?.items || []
      total.value = body.data?.total || 0
    } catch (e) {
      error.value = e instanceof Error ? e.message : '请求失败'
    } finally {
      loading.value = false
    }
  }

  function onPageChange(page: number, size: number) {
    currentPage.value = page
    pageSize.value = size
    fetch()
  }

  return { loading, error, data, total, currentPage, pageSize, fetch, onPageChange }
}
