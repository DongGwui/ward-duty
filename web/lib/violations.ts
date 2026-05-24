// rule_id → 한국어 메시지 매핑 (Design §9.5).
//
// 솔버/Go API가 보내는 영문 메시지가 있긴 하나, UI에선 일관된 한국어 단문으로 표시.

export const RULE_MESSAGES: Record<string, { title: string; desc?: string; severity: "hard" | "soft" }> = {
  // Hard
  "H-01": { title: "N 다음날 D 금지", severity: "hard" },
  "H-02": { title: "N 연속 한도 초과", severity: "hard" },
  "H-03": { title: "연속 근무 5일 초과", severity: "hard" },
  "H-04": { title: "시프트 최소 인원 미달", severity: "hard" },
  "H-05": { title: "불가일에 배정됨", severity: "hard" },
  "H-06": { title: "N 후 휴식 부족", severity: "hard" },
  "H-10": { title: "이전 달 경계 위반", severity: "hard" },
  "H-11": { title: "고정 패턴 위반", severity: "hard" },
  "H-12": { title: "등급별 최소 인원 미달", severity: "hard" },
  "H-13": { title: "DE 자동 생성 불가", severity: "hard" },
  "H-14": { title: "DE 다음날 D 금지", severity: "hard" },
  // Night-Keeper
  "K-01": { title: "나이트킵은 N/O만 가능", severity: "hard" },
  "K-02": { title: "3달 연속 지정 금지", severity: "hard" },
  "K-04": { title: "cooldown 3달 필요", severity: "hard" },
  "K-05": { title: "고정 패턴과 충돌", severity: "hard" },
  // Soft
  "S-01": { title: "Off 균형", severity: "soft" },
  "S-02": { title: "희망일 반영", severity: "soft" },
  "S-03": { title: "주말 균형", severity: "soft" },
  "S-04": { title: "같은 시프트 연속", severity: "soft" },
  "S-05": { title: "짧은 휴식 패턴", severity: "soft" },
  "S-10": { title: "등급별 배정 비용", severity: "soft" },
  // Swap
  "X-03": { title: "Swap 결과 규칙 위반", severity: "hard" },
};

export function getRuleMessage(ruleId: string): { title: string; severity: "hard" | "soft" } {
  return RULE_MESSAGES[ruleId] ?? { title: ruleId, severity: "hard" };
}
