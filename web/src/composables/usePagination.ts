import { ref } from 'vue'

export function usePagination(fetchFn: (page: number, size: number) => Promise<any>, defaultSize = 20) {
  const loading = ref(false)
  const data = ref<any[]>([])
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(defaultSize)

  async function fetch(page?: number) {
    if (page !== undefined) currentPage.value = page
    loading.value = true
    try {
      const resp = await fetchFn(currentPage.value, pageSize.value)
      const body = resp.data
      data.value = body.data?.items || body.data || []
      total.value = body.data?.total || 0
    } finally {
      loading.value = false
    }
  }

  function onPageChange(page: number, size: number) {
    currentPage.value = page
    pageSize.value = size
    fetch()
  }

  return { loading, data, total, currentPage, pageSize, fetch, onPageChange }
}
