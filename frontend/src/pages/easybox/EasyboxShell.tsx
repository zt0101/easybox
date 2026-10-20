import { useEffect, useMemo, type ReactNode } from 'react';
import { ConfigProvider, Layout, Modal, message } from 'antd';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { setMessageInstance } from '@/utils/messageBus';
import './easybox.css';

export interface EasyboxPageApi {
  messageApi: ReturnType<typeof message.useMessage>[0];
  modal: ReturnType<typeof Modal.useModal>[0];
}

/* Same shell as the upstream pages (theme, sidebar, message bus), shared by
 * the three EasyBox pages so each only renders its own content. */
export default function EasyboxShell({
  children,
}: {
  children: (api: EasyboxPageApi) => ReactNode;
}) {
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const pageClass = useMemo(() => {
    const classes = ['easybox-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <div className="easybox-content">{children({ messageApi, modal })}</div>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
