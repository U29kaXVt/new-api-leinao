/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsDonation(props) {
  const { t } = useTranslation();
  const refForm = useRef(null);
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'donation_setting.enabled': false,
    'donation_setting.template_channel_id': 0,
    'donation_setting.reward_quota': 0,
    'donation_setting.guide_text': '',
    'donation_setting.copy_button_text': '',
    'donation_setting.copy_button_content': '',
  });

  const handleFieldChange = (field) => {
    return (value) => {
      setInputs((prev) => ({ ...prev, [field]: value }));
    };
  };

  const onSubmit = async () => {
    const templateChannelID = Number(
      inputs['donation_setting.template_channel_id'] || 0,
    );
    const rewardQuota = Number(inputs['donation_setting.reward_quota'] || 0);
    const guideText = inputs['donation_setting.guide_text'] || '';
    const copyButtonText = inputs['donation_setting.copy_button_text'] || '';
    const copyButtonContent =
      inputs['donation_setting.copy_button_content'] || '';

    if (templateChannelID < 0) {
      showError(t('模板渠道 ID 不能小于 0'));
      return;
    }
    if (rewardQuota < 0) {
      showError(t('捐赠奖励额度不能小于 0'));
      return;
    }

    setLoading(true);
    try {
      await Promise.all([
        API.put('/api/option/', {
          key: 'donation_setting.enabled',
          value: String(inputs['donation_setting.enabled']),
        }),
        API.put('/api/option/', {
          key: 'donation_setting.template_channel_id',
          value: String(templateChannelID),
        }),
        API.put('/api/option/', {
          key: 'donation_setting.reward_quota',
          value: String(rewardQuota),
        }),
        API.put('/api/option/', {
          key: 'donation_setting.guide_text',
          value: guideText,
        }),
        API.put('/api/option/', {
          key: 'donation_setting.copy_button_text',
          value: copyButtonText,
        }),
        API.put('/api/option/', {
          key: 'donation_setting.copy_button_content',
          value: copyButtonContent,
        }),
      ]);
      showSuccess(t('保存成功'));
      props.refresh?.();
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const nextInputs = {
      'donation_setting.enabled':
        props.options?.['donation_setting.enabled'] === true ||
        props.options?.['donation_setting.enabled'] === 'true',
      'donation_setting.template_channel_id': Number(
        props.options?.['donation_setting.template_channel_id'] || 0,
      ),
      'donation_setting.reward_quota': Number(
        props.options?.['donation_setting.reward_quota'] || 0,
      ),
      'donation_setting.guide_text':
        props.options?.['donation_setting.guide_text'] || '',
      'donation_setting.copy_button_text':
        props.options?.['donation_setting.copy_button_text'] || '',
      'donation_setting.copy_button_content':
        props.options?.['donation_setting.copy_button_content'] || '',
    };
    setInputs(nextInputs);
    refForm.current?.setValues(nextInputs);
  }, [props.options]);

  return (
    <Spin spinning={loading}>
      <Form
        values={inputs}
        getFormApi={(formApi) => (refForm.current = formApi)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('捐赠渠道设置')}>
          <Typography.Text
            type='tertiary'
            style={{ marginBottom: 16, display: 'block' }}
          >
            {t(
              '普通用户提交 base_url 和 key 后，系统会基于模板渠道克隆出专属捐赠渠道，仅用于调用资格校验，不参与全局选路。',
            )}
          </Typography.Text>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field='donation_setting.enabled'
                label={t('启用捐赠渠道')}
                checkedText='｜'
                uncheckedText='〇'
                onChange={handleFieldChange('donation_setting.enabled')}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='donation_setting.template_channel_id'
                label={t('模板渠道 ID')}
                min={0}
                placeholder={t('管理员手工创建的模板渠道 ID')}
                onChange={handleFieldChange(
                  'donation_setting.template_channel_id',
                )}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='donation_setting.reward_quota'
                label={t('单条有效捐赠奖励额度')}
                min={0}
                placeholder={t('捐赠校验成功后发放的一次性额度')}
                onChange={handleFieldChange('donation_setting.reward_quota')}
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={24}>
              <Form.TextArea
                field='donation_setting.guide_text'
                label={t('捐赠指引')}
                placeholder={t('展示在用户捐赠页面顶部的说明内容，支持多行文本')}
                autosize
                onChange={handleFieldChange('donation_setting.guide_text')}
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <Form.Input
                field='donation_setting.copy_button_text'
                label={t('复制按钮文案')}
                placeholder={t('例如：复制捐赠交流群')}
                onChange={handleFieldChange(
                  'donation_setting.copy_button_text',
                )}
              />
            </Col>
            <Col xs={24} sm={12}>
              <Form.TextArea
                field='donation_setting.copy_button_content'
                label={t('复制按钮内容')}
                placeholder={t('点击按钮后写入剪贴板的内容，支持多行文本')}
                autosize
                onChange={handleFieldChange(
                  'donation_setting.copy_button_content',
                )}
              />
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存捐赠设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
