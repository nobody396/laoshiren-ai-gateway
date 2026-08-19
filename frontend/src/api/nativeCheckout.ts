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

export type NativeCheckoutProvider = 'ldxp' | 'easypay'

export interface NativeCheckoutOrder {
  order_no: string
  status: NativeCheckoutStatus
  provider?: NativeCheckoutProvider
  product_kind?: 'balance' | 'subscription'
  pay_amount_cny_fen: number
  benefit_amount_cny_fen: number
  redeem_validity_days?: number
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
  redeem_validity_days?: number
  once_per_user: boolean
  claimed: boolean
  provider?: NativeCheckoutProvider
  order?: NativeCheckoutOrder
}

export interface NativeCheckoutManualOfferStatus {
  code: string
  claimed: boolean
  purchase_url?: string
}

export async function listNativeCheckoutOffers(): Promise<NativeCheckoutOffer[]> {
  const { data } = await apiClient.get<NativeCheckoutOffer[]>('/native-checkout/offers')
  return data
}

export async function getNativeCheckoutManualOfferStatus(offerCode: string): Promise<NativeCheckoutManualOfferStatus> {
  const { data } = await apiClient.get<NativeCheckoutManualOfferStatus>(
    `/native-checkout/manual-offers/${encodeURIComponent(offerCode)}`,
  )
  return data
}

export async function requestNativeCheckoutManualOfferPurchase(offerCode: string): Promise<NativeCheckoutManualOfferStatus> {
  const { data } = await apiClient.post<NativeCheckoutManualOfferStatus>(
    `/native-checkout/manual-offers/${encodeURIComponent(offerCode)}/purchase`,
  )
  return data
}

export async function createNativeCheckoutOrder(
  offerCode: string,
  payType?: NativeCheckoutPaymentMethod
): Promise<NativeCheckoutOrder> {
  const { data } = await apiClient.post<NativeCheckoutOrder>('/native-checkout/orders', {
    offer_code: offerCode,
    // 仅 easypay 通道需要指定支付方式；缺省时后端按支付宝处理。
    ...(payType ? { pay_type: payType } : {}),
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
