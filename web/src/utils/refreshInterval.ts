/**
 * 将Go duration字符串转换为毫秒数
 * 例如: "5m0s" -> 300000, "3m0s" -> 180000, "5m" -> 300000
 */
export function parseDurationToMs(durationStr: string): number {
  if (!durationStr) return 300000 // 默认5分钟

  // 移除空格
  durationStr = durationStr.trim()

  let totalMs = 0

  // 解析小时 (h)
  const hoursMatch = durationStr.match(/(\d+)h/)
  if (hoursMatch) {
    totalMs += parseInt(hoursMatch[1]) * 60 * 60 * 1000
  }

  // 解析分钟 (m)
  const minutesMatch = durationStr.match(/(\d+)m/)
  if (minutesMatch) {
    totalMs += parseInt(minutesMatch[1]) * 60 * 1000
  }

  // 解析秒 (s) - 注意：如果已经有分钟，秒数可能是小数部分
  const secondsMatch = durationStr.match(/(\d+)(?:\.\d+)?s/)
  if (secondsMatch) {
    totalMs += Math.floor(parseFloat(secondsMatch[1])) * 1000
  }

  // 如果解析失败，尝试直接解析数字（假设是秒数）
  if (totalMs === 0) {
    const numberMatch = durationStr.match(/(\d+)/)
    if (numberMatch) {
      // 如果只是数字，假设是秒数
      totalMs = parseInt(numberMatch[1]) * 1000
    }
  }

  // 如果仍然解析失败，返回默认值（5分钟）
  if (totalMs === 0) {
    console.warn(`无法解析duration字符串: ${durationStr}，使用默认值5分钟`)
    return 300000
  }

  return totalMs
}

/**
 * 根据AI调用周期计算前端刷新间隔
 * 刷新间隔 = AI周期 + 5秒（给AI处理时间）
 * 
 * @param scanInterval - AI扫描间隔（Go duration字符串，如 "5m0s"）
 * @param bufferSeconds - 缓冲时间（秒），默认5秒
 * @returns 刷新间隔（毫秒）
 */
export function calculateRefreshInterval(
  scanInterval: string | undefined,
  bufferSeconds: number = 5
): number {
  if (!scanInterval) {
    // 如果没有扫描间隔，使用默认值（5分钟 + 5秒）
    return 300000 + bufferSeconds * 1000
  }

  const intervalMs = parseDurationToMs(scanInterval)
  const refreshMs = intervalMs + bufferSeconds * 1000

  // 最小刷新间隔为10秒
  return Math.max(refreshMs, 10000)
}

/**
 * 获取不同数据类型的刷新间隔
 * - 实时数据（账户、持仓、状态）：AI周期 + 5秒
 * - 决策数据：AI周期 + 5秒（与实时数据同步）
 * - 统计数据：AI周期 * 2（更新频率较低）
 * - 历史数据：AI周期 * 2（更新频率较低）
 */
export function getRefreshIntervals(scanInterval: string | undefined): {
  realtime: number // 实时数据刷新间隔
  decision: number // 决策数据刷新间隔
  statistics: number // 统计数据刷新间隔
  history: number // 历史数据刷新间隔
} {
  const baseInterval = calculateRefreshInterval(scanInterval, 5)

  return {
    realtime: baseInterval, // 实时数据：AI周期 + 5秒
    decision: baseInterval, // 决策数据：AI周期 + 5秒（与实时数据同步）
    statistics: baseInterval * 2, // 统计数据：AI周期 * 2
    history: baseInterval * 2, // 历史数据：AI周期 * 2
  }
}

