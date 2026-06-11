import { ref } from 'vue'
import { getMonthlyCardStatus } from '@/api/monthlyCardStatus'
import {
  buildMonthlyCreditCardPlans,
  monthlyCreditCardPlans as fallbackMonthlyCreditCardPlans,
  type MonthlyCreditCardPlan
} from '@/constants/monthlyCreditCards'

export function useMonthlyCreditCardPlans() {
  const plans = ref<MonthlyCreditCardPlan[]>(fallbackMonthlyCreditCardPlans)
  const loading = ref(false)

  async function loadMonthlyCreditCardPlans() {
    loading.value = true
    try {
      const snapshot = await getMonthlyCardStatus(60)
      plans.value = buildMonthlyCreditCardPlans(snapshot.plans)
    } catch (error) {
      plans.value = fallbackMonthlyCreditCardPlans
    } finally {
      loading.value = false
    }
  }

  return {
    plans,
    loading,
    loadMonthlyCreditCardPlans
  }
}
