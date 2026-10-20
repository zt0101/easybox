import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { Button, Card, Empty, Result, Space, Switch, Table, Tag, Tooltip, Typography } from 'antd';
import type { TableColumnsType } from 'antd';
import {
  CopyOutlined,
  DeleteOutlined,
  PlusOutlined,
  QrcodeOutlined,
  SyncOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';

import { ClipboardManager, SizeFormatter } from '@/utils';
import {
  fetchEndpointLink,
  useEasyboxEndpoints,
  useEasyboxMutations,
} from '@/api/queries/useEasybox';
import type { EndpointView } from '@/schemas/api/easybox';
import EasyboxShell, { type EasyboxPageApi } from './EasyboxShell';
import LinkQrModal from './LinkQrModal';
import RegenerateModal from './RegenerateModal';

const STATUS_COLOR: Record<string, string> = {
  RUNNING: 'success',
  DISABLED: 'default',
  ERROR: 'error',
  CREATING: 'processing',
  DELETING: 'warning',
};

export default function LinesPage() {
  return <EasyboxShell>{(api) => <Lines {...api} />}</EasyboxShell>;
}

function Lines({ messageApi, modal }: EasyboxPageApi) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const lines = useEasyboxEndpoints();
  const { remove, setEnable, regenerate, regenerating } = useEasyboxMutations();
  const [qr, setQr] = useState<{ name: string; link: string } | null>(null);
  const [regenLine, setRegenLine] = useState<EndpointView | null>(null);

  const confirmRegenerate = async (keepOld: boolean) => {
    if (!regenLine) return;
    const result = await regenerate(regenLine.id, keepOld);
    setRegenLine(null);
    if (result.ok) messageApi.success(t('easybox.lines.regenerated'));
    else messageApi.error(t('easybox.lines.regenerateFailed', { reason: result.message }));
  };

  const copy = async (text: string) => {
    if (text && (await ClipboardManager.copyText(text))) messageApi.success(t('copied'));
  };

  const confirmDelete = (line: EndpointView) =>
    modal.confirm({
      title: t('easybox.lines.deleteConfirm', { name: line.name }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(line.id);
        if (msg?.success) messageApi.success(t('easybox.lines.deleted'));
      },
    });

  const hasInbound = (line: EndpointView) => line.inboundId > 0 && line.status !== 'ERROR';

  const columns: TableColumnsType<EndpointView> = [
    {
      title: t('easybox.lines.colName'),
      key: 'name',
      render: (_, line) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{line.name}</Typography.Text>
          {line.port > 0 && (
            <span className="easybox-mono">
              {(line.address || window.location.hostname) + ':' + line.port}
            </span>
          )}
        </Space>
      ),
    },
    {
      title: t('easybox.lines.colProtocol'),
      key: 'protocol',
      render: (_, line) => line.protocol.toUpperCase(),
    },
    {
      title: t('easybox.lines.colStatus'),
      key: 'status',
      render: (_, line) => (
        <Space size={4} wrap>
          <Tooltip title={line.status === 'ERROR' ? line.lastError : undefined}>
            <Tag color={STATUS_COLOR[line.status] ?? 'default'}>
              {t(`easybox.status.${line.status}`)}
            </Tag>
          </Tooltip>
          {hasInbound(line) && !line.inSubscription && (
            <Tag>{t('easybox.lines.notInSubscription')}</Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('easybox.lines.colTraffic'),
      key: 'traffic',
      render: (_, line) => SizeFormatter.sizeFormat(line.up + line.down),
    },
    {
      title: t('easybox.lines.colActions'),
      key: 'actions',
      render: (_, line) => (
        <Space wrap>
          <Tooltip title={t('easybox.lines.copyLink')}>
            <Button
              icon={<CopyOutlined />}
              aria-label={t('easybox.lines.copyLink')}
              disabled={!hasInbound(line)}
              onClick={async () => copy(await fetchEndpointLink(line.id))}
            />
          </Tooltip>
          <Tooltip title={t('easybox.lines.qr')}>
            <Button
              icon={<QrcodeOutlined />}
              aria-label={t('easybox.lines.qr')}
              disabled={!hasInbound(line)}
              onClick={async () => {
                const link = await fetchEndpointLink(line.id);
                if (link) setQr({ name: line.name, link });
              }}
            />
          </Tooltip>
          <Tooltip title={t('easybox.lines.regenerate')}>
            <Button
              icon={<SyncOutlined />}
              aria-label={t('easybox.lines.regenerate')}
              disabled={!hasInbound(line)}
              onClick={() => setRegenLine(line)}
            />
          </Tooltip>
          <Tooltip
            title={
              line.status === 'DISABLED' ? t('easybox.lines.enable') : t('easybox.lines.disable')
            }
          >
            <Switch
              size="small"
              checked={line.status !== 'DISABLED'}
              disabled={!hasInbound(line) && line.status !== 'DISABLED'}
              onChange={(next) => setEnable(line.id, next)}
            />
          </Tooltip>
          <Tooltip title={t('delete')}>
            <Button
              danger
              icon={<DeleteOutlined />}
              aria-label={t('delete')}
              onClick={() => confirmDelete(line)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];

  if (lines.isError) {
    return (
      <Result
        status="error"
        title={t('somethingWentWrong')}
        subTitle={(lines.error as Error).message}
        extra={<Button onClick={() => lines.refetch()}>{t('refresh')}</Button>}
      />
    );
  }

  const data = lines.data ?? [];
  return (
    <Card
      title={
        <Space>
          <UnorderedListOutlined />
          {t('easybox.lines.title')}
        </Space>
      }
      extra={
        data.length > 0 && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate('/easybox/create')}
          >
            {t('easybox.lines.create')}
          </Button>
        )
      }
    >
      {!lines.isLoading && data.length === 0 ? (
        <Empty
          description={
            <Space direction="vertical" size={2}>
              <Typography.Text strong>{t('easybox.lines.emptyTitle')}</Typography.Text>
              <Typography.Text type="secondary">{t('easybox.lines.emptyHint')}</Typography.Text>
            </Space>
          }
        >
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate('/easybox/create')}
          >
            {t('easybox.lines.emptyAction')}
          </Button>
        </Empty>
      ) : (
        <Table<EndpointView>
          rowKey="id"
          size="middle"
          loading={lines.isLoading}
          columns={columns}
          dataSource={data}
          pagination={false}
          scroll={{ x: 'max-content' }}
        />
      )}
      <LinkQrModal
        open={qr !== null}
        title={t('easybox.lines.qrTitle', { name: qr?.name ?? '' })}
        hint={t('easybox.lines.qrHint')}
        value={qr?.link ?? ''}
        onCopy={copy}
        onClose={() => setQr(null)}
      />
      <RegenerateModal
        name={regenLine?.name ?? null}
        loading={regenerating}
        onConfirm={confirmRegenerate}
        onClose={() => setRegenLine(null)}
      />
    </Card>
  );
}
