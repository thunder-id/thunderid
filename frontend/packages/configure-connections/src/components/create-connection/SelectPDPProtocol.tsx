// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {ServerCog} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SelectableConnectionCard from './SelectableConnectionCard';
import {type ConnectionType, ConnectionTypes} from '../../models/connection';

interface SelectPDPProtocolProps {
  selectedProtocol: ConnectionType | null;
  onSelect: (protocol: ConnectionType) => void;
}

export default function SelectPDPProtocol({selectedProtocol, onSelect}: SelectPDPProtocolProps): JSX.Element {
  const {t} = useTranslation('connections');
  const selected: boolean = selectedProtocol === ConnectionTypes.AUTHZEN_PDP;

  return (
    <Stack direction="column" spacing={3} data-testid="select-pdp-protocol">
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1">{t('wizard.protocol.heading', 'Choose a policy decision point protocol')}</Typography>
        <Typography variant="body1" color="text.secondary">
          {t('wizard.protocol.subheading', 'Select the protocol supported by your policy decision point.')}
        </Typography>
      </Stack>
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: 'repeat(2, 1fr)'}, gap: 2}}>
        <SelectableConnectionCard
          description={t(
            'wizard.protocol.authzen.description',
            'Use the AuthZEN protocol to send authorization evaluation requests.',
          )}
          icon={<ServerCog size={28} />}
          label={t('wizard.protocol.authzen.label', 'AuthZEN')}
          selected={selected}
          testId="pdp-protocol-option-authzen"
          onSelect={() => onSelect(ConnectionTypes.AUTHZEN_PDP)}
        />
      </Box>
    </Stack>
  );
}
