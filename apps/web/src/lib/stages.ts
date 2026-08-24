import type { ApplicationStage } from '@recruitmate/shared-types';
import { STAGE_LABELS, STAGE_TRANSITIONS } from '@recruitmate/shared-types';

export interface StageOption {
  value: ApplicationStage;
  label: string;
}

/** 全部阶段选项（按 STAGE_LABELS 顺序） */
export const STAGE_OPTIONS: StageOption[] = (
  Object.entries(STAGE_LABELS) as [ApplicationStage, string][]
).map(([value, label]) => ({ value, label }));

/**
 * 阶段流转下拉选项：仅保留当前阶段自身 + STAGE_TRANSITIONS 定义的合法流转目标。
 * 例如 interview 阶段可选：interview（自身）/ offer_pending / rejected。
 */
export function stageOptionsFor(current: ApplicationStage): StageOption[] {
  const allowed = STAGE_TRANSITIONS[current];
  return STAGE_OPTIONS.filter((o) => o.value === current || allowed.includes(o.value));
}
