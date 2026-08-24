import type { ApplicationInternal } from '@recruitmate/shared-types';
import { Form, Input, Modal, message } from 'antd';
import { errorMessage, useCreateOffer } from '../hooks/useApi';

interface OfferCreateModalProps {
  open: boolean;
  application: ApplicationInternal | null;
  onClose: () => void;
}

/**
 * 发起 Offer 审批弹窗（HR/管理员，候选人须处于部门负责人面阶段）。
 * 建议薪资 / 入职时间 / 备注均为可选；最终薪资由部门负责人审批时确定。
 * 我的待办（offer_ready）、候选人抽屉（MatchDrawer）共用。
 */
export function OfferCreateModal({ open, application, onClose }: OfferCreateModalProps) {
  const [form] = Form.useForm();
  const createOfferMutation = useCreateOffer();

  const confirm = async () => {
    if (!application) return;
    let values: { salary?: string; joinDate?: string; note?: string } = {};
    try {
      values = (await form.validateFields()) as { salary?: string; joinDate?: string; note?: string };
    } catch {
      return; // 表单校验未通过
    }
    try {
      await createOfferMutation.mutateAsync({ id: application.id, body: values });
      message.success('Offer 审批已提交');
      onClose();
    } catch (err) {
      message.error(errorMessage(err, '发起 Offer 失败'));
    }
  };

  return (
    <Modal
      title={`发起 Offer 审批 · ${application?.candidateName ?? ''}`}
      open={open}
      onOk={confirm}
      onCancel={onClose}
      okText="提交审批"
      cancelText="取消"
      confirmLoading={createOfferMutation.isPending}
      destroyOnHidden
    >
      <Form form={form} layout="vertical">
        <Form.Item name="salary" label="建议薪资（千元/月）" extra="最终薪资由部门负责人审批时确定">
          <Input placeholder="如 20-25（可选）" />
        </Form.Item>
        <Form.Item name="joinDate" label="入职时间">
          <Input placeholder="如 2025-03-01（可选）" />
        </Form.Item>
        <Form.Item name="note" label="备注">
          <Input.TextArea rows={3} placeholder="Offer 补充说明（可选）" maxLength={500} showCount />
        </Form.Item>
      </Form>
    </Modal>
  );
}
