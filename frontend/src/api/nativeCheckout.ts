import { apiClient } from './client'

export type NativeCheckoutStatus =
  | 'creating'
  | 'pending'
  | 'checking'
  | 'fulfilling'
  | 'completed'
  | 'failed'
  | 'manual_review'

export type NativeCheckoutPaymentMethod = 'wechat' | 'alipay'

export interface NativeCheckoutOrder {
  order_no: string
  status: NativeCheckoutStatus
  pay_amount_cny_fen: number
  benefit_amount_cny_fen: number
  payment_url?: string
  payment_method?: NativeCheckoutPaymentMethod
  direct_qr_url?: string
  created_at: string
}

export interface NativeCheckoutOffer {
  code: string
  name: string
  description: string
  product_kind: 'balance' | 'subscription'
  pay_amount_cny_fen: number
  benefit_amount_cny_fen: number
  once_per_user: boolean
  claimed: boolean
  order?: NativeCheckoutOrder
}

export async function listNativeCheckoutOffers(): Promise<NativeCheckoutOffer[]> {
  const { data } = await apiClient.get<NativeCheckoutOffer[]>('/native-checkout/offers')
  return data
}

export async function createNativeCheckoutOrder(offerCode: string): Promise<NativeCheckoutOrder> {
  const { data } = await apiClient.post<NativeCheckoutOrder>('/native-checkout/orders', {
    offer_code: offerCode,
  })
  return data
}

export async function getNativeCheckoutOrder(orderNo: string): Promise<NativeCheckoutOrder> {
  const { data } = await apiClient.get<NativeCheckoutOrder>(`/native-checkout/orders/${encodeURIComponent(orderNo)}`)
  return data
}

export async function getNativeCheckoutDirectQR(orderNo: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(
    `/native-checkout/orders/${encodeURIComponent(orderNo)}/qr`,
    { responseType: 'blob' },
  )
  return data
}
