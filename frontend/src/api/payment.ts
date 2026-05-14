/**
 * Payment API endpoints
 * Handles Stripe Checkout payment for balance top-up
 */

import { apiClient } from './client'

export interface PriceTier {
  amount_cents: number
  credits: number
  label: string
}

export interface CheckoutResponse {
  checkout_url: string
  order_no: string
}

export interface PaymentOrder {
  id: number
  order_no: string
  user_id: number
  amount_cents: number
  credits: number
  currency: string
  status: string
  stripe_session_id?: string
  stripe_payment_intent_id?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

/**
 * Get available price tiers
 */
export async function getPriceTiers(): Promise<PriceTier[]> {
  const { data } = await apiClient.get<PriceTier[]>('/payment/tiers')
  return data
}

/**
 * Create a Stripe Checkout session
 */
export async function createCheckoutSession(
  amountCents: number,
  successUrl: string,
  cancelUrl: string
): Promise<CheckoutResponse> {
  const { data } = await apiClient.post<CheckoutResponse>('/payment/checkout', {
    amount_cents: amountCents,
    success_url: successUrl,
    cancel_url: cancelUrl
  })
  return data
}

/**
 * Get user's payment history
 */
export async function getPaymentHistory(): Promise<PaymentOrder[]> {
  const { data } = await apiClient.get<PaymentOrder[]>('/payment/history')
  return data
}

export const paymentAPI = {
  getPriceTiers,
  createCheckoutSession,
  getPaymentHistory
}

export default paymentAPI
