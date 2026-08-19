import QRCode from 'qrcode'

const DIRECT_QR_IMAGE_PATTERN = /^https?:\/\//i

/**
 * 充值/收银台返回的二维码载荷可能是可直接用于 <img> 的图片地址，
 * 也可能是需要客户端编码的支付内容（如 weixin:// 协议或收银台链接）。
 */
export function isDirectQrImageUrl(payload: string): boolean {
  return DIRECT_QR_IMAGE_PATTERN.test(payload.trim())
}

/** 将支付内容渲染为二维码 data URL（保持黑码白底，保证可扫描）。 */
export async function renderQrCodeDataUrl(payload: string): Promise<string> {
  return QRCode.toDataURL(payload.trim(), {
    errorCorrectionLevel: 'M',
    margin: 2,
    width: 320,
  })
}
