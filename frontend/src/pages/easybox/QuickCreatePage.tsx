import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { FormProvider } from 'react-hook-form';
import { Alert, Button, Card, Form, Input, Space, Steps, Typography } from 'antd';
import { CopyOutlined, ThunderboltOutlined, UnorderedListOutlined } from '@ant-design/icons';

import { FormField, rhfZodValidate, useZodForm } from '@/components/form/rhf';
import { ClipboardManager } from '@/utils';
import {
  fetchEndpointLink,
  useEasyboxMutations,
  useEasyboxSubscription,
  useEasyboxTemplates,
  type CreateResult,
} from '@/api/queries/useEasybox';
import { QuickCreateFormSchema, type QuickCreateFormValues } from '@/schemas/api/easybox';
import EasyboxShell, { type EasyboxPageApi } from './EasyboxShell';

// Quick Create's steps in the order the backend runs them (EasyBox.md §12).
const CREATE_STEPS = ['prepare', 'create_inbound', 'apply_runtime', 'health_check'] as const;

export default function QuickCreatePage() {
  return <EasyboxShell>{(api) => <QuickCreate {...api} />}</EasyboxShell>;
}

function QuickCreate({ messageApi, modal }: EasyboxPageApi) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { create, creating } = useEasyboxMutations();
  const subscription = useEasyboxSubscription();
  const templates = useEasyboxTemplates();
  const [template, setTemplate] = useState('vless-reality');
  const [result, setResult] = useState<CreateResult | null>(null);
  const methods = useZodForm(QuickCreateFormSchema, { defaultValues: { name: '', address: '' } });

  const submit = methods.handleSubmit(async (values: QuickCreateFormValues) => {
    const run = async () => {
      setResult(null);
      setResult(await create({ ...values, template }));
    };
    // The first Shadowsocks 2022 line on an older subscription changes the
    // password TUIC / Trojan lines share, so devices must refresh: ask first.
    if (templates.data?.find((tpl) => tpl.id === template)?.passwordUpgrade) {
      modal.confirm({
        title: t('easybox.create.passwordUpgradeTitle'),
        content: t('easybox.create.passwordUpgradeContent'),
        okText: t('easybox.create.passwordUpgradeOk'),
        cancelText: t('cancel'),
        onOk: run,
      });
      return;
    }
    await run();
  });

  const copy = async (text: string) => {
    if (text && (await ClipboardManager.copyText(text))) messageApi.success(t('copied'));
  };

  const failedIndex = result?.line?.failedStep
    ? CREATE_STEPS.indexOf(result.line.failedStep as (typeof CREATE_STEPS)[number])
    : -1;

  return (
    <Card
      title={
        <Space>
          <ThunderboltOutlined />
          {t('easybox.create.title')}
        </Space>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Text strong>{t('easybox.create.protocol')}</Typography.Text>
          <div className="easybox-protocols" style={{ marginTop: 8 }}>
            {(templates.data ?? []).map((tpl) => (
              <button
                type="button"
                key={tpl.id}
                className={`easybox-protocol${tpl.id === template ? ' is-selected' : ''}${tpl.available ? '' : ' is-disabled'}`}
                aria-pressed={tpl.id === template}
                disabled={!tpl.available}
                onClick={() => setTemplate(tpl.id)}
              >
                <Typography.Text strong>{tpl.name}</Typography.Text>
                <span className="easybox-protocol-hint">
                  {tpl.available
                    ? t(`easybox.templates.${tpl.id}`)
                    : t('easybox.create.needsCertificate')}
                </span>
              </button>
            ))}
          </div>
        </div>

        {templates.data?.some((tpl) => !tpl.available) && (
          <Alert
            type="info"
            showIcon
            message={t('easybox.create.certificateGuide')}
            action={
              <Button size="small" onClick={() => navigate('/settings')}>
                {t('easybox.create.setCertificate')}
              </Button>
            }
          />
        )}

        <FormProvider {...methods}>
          <form onSubmit={submit}>
            <Form layout="vertical" component={false}>
              <FormField
                name="name"
                label={t('easybox.create.name')}
                rules={{ validate: rhfZodValidate(QuickCreateFormSchema.shape.name) }}
              >
                <Input maxLength={64} placeholder={t('easybox.create.namePlaceholder')} />
              </FormField>
              <FormField
                name="address"
                label={t('easybox.create.address')}
                extra={t('easybox.create.addressHelp')}
                rules={{ validate: rhfZodValidate(QuickCreateFormSchema.shape.address) }}
              >
                <Input maxLength={253} placeholder="node.example.com" dir="ltr" />
              </FormField>
              <Button type="primary" htmlType="submit" size="large" loading={creating}>
                {t('easybox.create.submit')}
              </Button>
            </Form>
          </form>
        </FormProvider>

        {creating && <Alert type="info" showIcon message={t('easybox.create.creating')} />}

        {result?.ok && result.line && (
          <Alert
            type="success"
            showIcon
            message={t('easybox.create.successTitle', { name: result.line.name })}
            description={
              <Space direction="vertical">
                <span>
                  {t('easybox.create.firewallHint', {
                    port: result.line.port,
                    transport: result.line.transport.toUpperCase(),
                  })}
                </span>
                <Space wrap>
                  <Button
                    icon={<CopyOutlined />}
                    onClick={async () => copy(await fetchEndpointLink(result.line!.id))}
                  >
                    {t('easybox.lines.copyLink')}
                  </Button>
                  <Button
                    type="primary"
                    icon={<CopyOutlined />}
                    disabled={!subscription.data?.subUrl}
                    onClick={() => copy(subscription.data?.subUrl ?? '')}
                  >
                    {t('easybox.create.copySubscription')}
                  </Button>
                  <Button
                    icon={<UnorderedListOutlined />}
                    onClick={() => navigate('/easybox/lines')}
                  >
                    {t('easybox.create.goLines')}
                  </Button>
                </Space>
              </Space>
            }
          />
        )}

        {result && !result.ok && (
          <Alert
            type="error"
            showIcon
            message={t('easybox.create.failedTitle')}
            description={
              <Space direction="vertical" style={{ width: '100%' }}>
                {failedIndex >= 0 && (
                  <Steps
                    size="small"
                    current={failedIndex}
                    status="error"
                    items={CREATE_STEPS.map((s) => ({ title: t(`easybox.steps.${s}`) }))}
                  />
                )}
                <span>
                  {t('easybox.create.failedReason', {
                    reason: result.line?.lastError || result.message,
                  })}
                </span>
                <span>{t('easybox.create.failedCleanup')}</span>
                <Button onClick={submit} loading={creating}>
                  {t('easybox.create.retry')}
                </Button>
              </Space>
            }
          />
        )}
      </Space>
    </Card>
  );
}
