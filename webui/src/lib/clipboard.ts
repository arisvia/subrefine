/**
 * 统一剪贴板写入工具函数
 * 支持现代 navigator.clipboard API 以及纯 HTTP 局域网非安全上下文下的 execCommand 降级。
 * 返回 boolean 状态，由调用方自行决定提示文案与交互逻辑。
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // 失败时继续尝试 execCommand 降级
    }
  }

  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    textarea.style.top = '-9999px'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    const success = document.execCommand('copy')
    document.body.removeChild(textarea)
    return success
  } catch {
    return false
  }
}
