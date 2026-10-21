import { useTranslation } from 'react-i18next';
import { Button, Input, Modal, QRCode, Space } from 'antd';
import { CopyOutlined } from '@ant-design/icons';

interface LinkQrModalProps {
  open: boolean;
  title: string;
  hint: string;
  value: string;
  onCopy: (value: string) => void;
  onClose: () => void;
}

/* One modal for both QR kinds; the caller's title and hint keep "this line"
 * and "the whole subscription" apart so users scan the right one (§45). */
export default function LinkQrModal({
  open,
  title,
  hint,
  value,
  onCopy,
  onClose,
}: LinkQrModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={420}
      centered
      destroyOnHidden
      title={title}
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <span className="easybox-protocol-hint">{hint}</span>
        <div className="easybox-qr">
          <QRCode
            value={value || '-'}
            size={240}
            type="canvas"
            bordered={false}
            color="#000000"
            bgColor="#ffffff"
          />
        </div>
        <Input.TextArea
          className="easybox-mono"
          value={value}
          readOnly
          dir="ltr"
          autoSize={{ minRows: 2, maxRows: 5 }}
        />
        <Button type="primary" block icon={<CopyOutlined />} onClick={() => onCopy(value)}>
          {t('copy')}
        </Button>
      </Space>
    </Modal>
  );
}
