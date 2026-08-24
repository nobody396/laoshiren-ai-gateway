import { apiClient } from './client'

export type TopupPayType = 'alipay' | 'wechat'

export interface CreateTopupOrderResponse {
  order_no: string
  qr_code_url: string
  amount_cny_fen: number
  bonus_amount_cny_fen: number
  credited_amount_cny_fen: number
  pay_type: TopupPayType
}

export interface TopupOrderStatus {
  order_no: string
  status: 'pending' | 'completed' | 'expired'
  amount_cny_fen?: number
  bonus_amount_cny_fen?: number
  credited_amount_cny_fen?: number
  pay_type?: TopupPayType
  qr_code_url?: string | null
}

export interface TopupProductSelection {
  productAmountCnyFen: number
  quantity: number
}

/**
 * 创建充值订单
 * @param amountCnyFen 充值金额（分，CNY）。例如 2000 = ¥20
 * @param payType 支付渠道
 */
export async function createTopupOrder(
  amountCnyFen: number,
  payType: TopupPayType,
  product?: TopupProductSelection
): Promise<CreateTopupOrderResponse> {
  const { data } = await apiClient.post<CreateTopupOrderResponse>('/topup/order', {
    amount_cny_fen: amountCnyFen,
    pay_type: payType,
    ...(product ? {
      product_amount_cny_fen: product.productAmountCnyFen,
      quantity: product.quantity,
    } : {}),
  })
  return data
}

/**
 * 查询充值订单状态
 */
export async function queryTopupOrderStatus(orderNo: string): Promise<TopupOrderStatus> {
  const { data } = await apiClient.get<TopupOrderStatus>(`/topup/order/${orderNo}/status`)
  return data
}

export const topupAPI = {
  createTopupOrder,
  queryTopupOrderStatus,
}

export default topupAPI
