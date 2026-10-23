import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Empty,
  Input,
  Result,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';
import {
  CloudOutlined,
  CopyOutlined,
  LinkOutlined,
  QrcodeOutlined,
  TagsOutlined,
} from '@ant-design/icons';

import { ClipboardManager } from '@/utils';
import { useEasyboxMutations, useEasyboxSubscription } from '@/api/queries/useEasybox';
import EasyboxShell, { type EasyboxPageApi } from './EasyboxShell';
import LinkQrModal from './LinkQrModal';
import PreviewCard from './PreviewCard';

export default function SubscriptionPage() {
  return <EasyboxShell>{(api) => <Subscription {...api} />}</EasyboxShell>;
}

function UrlRow({
  label,
  value,
  onCopy,
  onQr,
}: {
  label: string;
  value: string;
  onCopy: () => void;
  onQr: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Space direction="vertical" size={4} style={{ width: '100%' }}>
      <Typography.Text strong>{label}</Typography.Text>
      <div className="easybox-sub-row">
        <Input className="easybox-sub-url easybox-mono" value={value} readOnly dir="ltr" />
        <Space>
          <Button type="primary" icon={<CopyOutlined />} onClick={onCopy}>
            {t('copy')}
          </Button>
          <Button icon={<QrcodeOutlined />} aria-label="QR" onClick={onQr} />
        </Space>
      </div>
    </Space>
  );
}

function Subscription({ messageApi, modal }: EasyboxPageApi) {
  const { t } = useTranslation();
  const sub = useEasyboxSubscription();
  const {
    setInSubscription,
    rotateSubscription,
    setSubscriptionCdn,
    settingSubscriptionCdn,
    setTierLabels,
    settingTierLabels,
  } = useEasyboxMutations();
  const [qrUrl, setQrUrl] = useState('');

  const copy = async (text: string) => {
    if (text && (await ClipboardManager.copyText(text))) messageApi.success(t('copied'));
  };

  const toggleCdn = async (enable: boolean) => {
    const res = await setSubscriptionCdn(enable);
    if (!res.ok) messageApi.error(res.message);
  };

  const toggleTierLabels = async (enable: boolean) => {
    const res = await setTierLabels(enable);
    if (!res.ok) messageApi.error(res.message);
  };

  const confirmRotate = () =>
    modal.confirm({
      title: t('easybox.subscription.rotateConfirmTitle'),
      content: t('easybox.subscription.rotateConfirm'),
      okText: t('easybox.subscription.rotate'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await rotateSubscription();
        if (msg?.success) messageApi.success(t('easybox.subscription.rotated'));
      },
    });

  if (sub.isError) {
    return (
      <Result
        status="error"
        title={t('somethingWentWrong')}
        subTitle={(sub.error as Error).message}
        extra={<Button onClick={() => sub.refetch()}>{t('refresh')}</Button>}
      />
    );
  }
  if (!sub.data) return <Spin size="large" />;

  const {
    subUrl,
    clashUrl,
    updateInterval,
    lines,
    cdnSubUrl,
    cdnClashUrl,
    cdnUnavailable,
    tierLabels,
  } = sub.data;
  const cdnOn = cdnSubUrl !== '';
  const visible = lines.filter((l): l is NonNullable<typeof l> => l !== null);
  return (
    <>
      <Card
        title={
          <Space>
            <LinkOutlined />
            {t('easybox.subscription.title')}
          </Space>
        }
      >
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <Typography.Text type="secondary">{t('easybox.subscription.hint')}</Typography.Text>
          {subUrl.startsWith('http://') && (
            <Alert type="warning" showIcon message={t('easybox.subscription.httpWarning')} />
          )}
          <UrlRow
            label={t('easybox.subscription.universal')}
            value={subUrl}
            onCopy={() => copy(subUrl)}
            onQr={() => setQrUrl(subUrl)}
          />
          {clashUrl && (
            <Space direction="vertical" size={2} style={{ width: '100%' }}>
              <UrlRow
                label={t('easybox.subscription.clash')}
                value={clashUrl}
                onCopy={() => copy(clashUrl)}
                onQr={() => setQrUrl(clashUrl)}
              />
              <Typography.Text type="secondary">
                {t('easybox.subscription.clashHint')}
              </Typography.Text>
            </Space>
          )}
          <Typography.Text type="secondary">
            {t('easybox.subscription.refreshHint', { hours: updateInterval })}
          </Typography.Text>
          <div>
            <Button danger onClick={confirmRotate}>
              {t('easybox.subscription.rotate')}
            </Button>
          </div>
        </Space>
      </Card>

      <Card
        title={
          <Space>
            <CloudOutlined />
            {t('easybox.subscription.cdnTitle')}
          </Space>
        }
        extra={
          <Switch
            checked={cdnOn}
            loading={settingSubscriptionCdn}
            disabled={!cdnOn && cdnUnavailable !== ''}
            onChange={toggleCdn}
            aria-label={t('easybox.subscription.cdnTitle')}
          />
        }
      >
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <Typography.Text type="secondary">{t('easybox.subscription.cdnIntro')}</Typography.Text>
          {!cdnOn && cdnUnavailable !== '' && (
            <Alert
              type="info"
              showIcon
              message={t('easybox.subscription.cdnUnavailable')}
              description={cdnUnavailable}
            />
          )}
          {cdnOn && (
            <>
              <UrlRow
                label={t('easybox.subscription.cdnUniversal')}
                value={cdnSubUrl}
                onCopy={() => copy(cdnSubUrl)}
                onQr={() => setQrUrl(cdnSubUrl)}
              />
              {cdnClashUrl && (
                <UrlRow
                  label={t('easybox.subscription.cdnClash')}
                  value={cdnClashUrl}
                  onCopy={() => copy(cdnClashUrl)}
                  onQr={() => setQrUrl(cdnClashUrl)}
                />
              )}
              <Alert type="success" showIcon message={t('easybox.subscription.cdnHowTo')} />
            </>
          )}
        </Space>
      </Card>

      <Card
        title={
          <Space>
            <TagsOutlined />
            {t('easybox.subscription.tierTitle')}
          </Space>
        }
        extra={
          <Switch
            checked={tierLabels}
            loading={settingTierLabels}
            onChange={toggleTierLabels}
            aria-label={t('easybox.subscription.tierTitle')}
          />
        }
      >
        <Typography.Text type="secondary">{t('easybox.subscription.tierIntro')}</Typography.Text>
      </Card>

      <Card title={t('easybox.subscription.lines')}>
        {visible.length === 0 ? (
          <Empty description={t('easybox.subscription.noLines')} />
        ) : (
          <Space direction="vertical" style={{ width: '100%' }}>
            {visible.map((line) => {
              const usable = line.inboundId > 0 && line.status !== 'ERROR';
              return (
                <Checkbox
                  key={line.id}
                  checked={usable && line.inSubscription}
                  disabled={!usable}
                  onChange={(e) => setInSubscription(line.id, e.target.checked)}
                >
                  <Space size={6}>
                    <Typography.Text>{line.name}</Typography.Text>
                    <Tag>{line.protocol.toUpperCase()}</Tag>
                    {line.status !== 'RUNNING' && <Tag>{t(`easybox.status.${line.status}`)}</Tag>}
                  </Space>
                </Checkbox>
              );
            })}
          </Space>
        )}
      </Card>

      <PreviewCard />

      <LinkQrModal
        open={qrUrl !== ''}
        title={t('easybox.subscription.qrTitle')}
        hint={t('easybox.subscription.qrHint')}
        value={qrUrl}
        onCopy={copy}
        onClose={() => setQrUrl('')}
      />
    </>
  );
}
