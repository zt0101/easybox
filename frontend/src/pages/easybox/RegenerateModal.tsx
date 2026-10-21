import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Radio, Space, Typography } from 'antd';

interface RegenerateModalProps {
  name: string | null;
  loading: boolean;
  onConfirm: (keepOld: boolean) => void;
  onClose: () => void;
}

// EasyBox.md §21: say up front that the current line survives until the new one checks out.
export default function RegenerateModal({
  name,
  loading,
  onConfirm,
  onClose,
}: RegenerateModalProps) {
  const { t } = useTranslation();
  const [keepOld, setKeepOld] = useState(false);
  return (
    <Modal
      open={name !== null}
      title={t('easybox.lines.regenerateTitle', { name: name ?? '' })}
      okText={t('easybox.lines.regenerate')}
      cancelText={t('cancel')}
      confirmLoading={loading}
      onOk={() => onConfirm(keepOld)}
      onCancel={onClose}
      destroyOnHidden
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Typography.Text type="secondary">{t('easybox.lines.regenerateHint')}</Typography.Text>
        <Radio.Group value={keepOld} onChange={(e) => setKeepOld(e.target.value as boolean)}>
          <Space direction="vertical">
            <Radio value={false}>
              <Typography.Text strong>{t('easybox.lines.modeReplace')}</Typography.Text>
              <div className="easybox-protocol-hint">{t('easybox.lines.modeReplaceHint')}</div>
            </Radio>
            <Radio value={true}>
              <Typography.Text strong>{t('easybox.lines.modeKeep')}</Typography.Text>
              <div className="easybox-protocol-hint">{t('easybox.lines.modeKeepHint')}</div>
            </Radio>
          </Space>
        </Radio.Group>
      </Space>
    </Modal>
  );
}
