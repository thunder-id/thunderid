// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {cleanup, fireEvent, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import {afterEach, describe, it, expect, vi} from 'vitest';
import renderWithProviders from '../../../../test/renderWithProviders';
import {openList, settle} from '../../../../test/selectInteractions';
import SelectAdapter, {type SelectAdapterOption, type SelectAdapterProps} from '../SelectAdapter';

afterEach(() => {
  cleanup();
});

const staticOptions = [
  {value: 'alpha', label: 'Alpha'},
  {value: 'beta', label: 'Beta'},
];

const baseProps: SelectAdapterProps = {
  component: {
    id: 'owner_input',
    type: 'SELECT',
    ref: 'owner',
    label: 'Owner',
    placeholder: 'Select an owner',
    options: staticOptions,
  },
  values: {},
  isLoading: false,
  resolve: (s) => s,
  onInputChange: vi.fn(),
};

const optionsOf = (count: number, from = 0): SelectAdapterOption[] =>
  Array.from({length: count}, (_, index) => ({value: `user-${from + index + 1}`, label: `User ${from + index + 1}`}));

describe('SelectAdapter', () => {
  describe('static options', () => {
    it('renders the label and the placeholder', () => {
      renderWithProviders(<SelectAdapter {...baseProps} />);

      expect(screen.getByText('Owner')).toBeTruthy();
      expect(screen.getByText('Select an owner')).toBeTruthy();
    });

    it('lists the component options', () => {
      renderWithProviders(<SelectAdapter {...baseProps} />);

      openList();

      expect(screen.getByRole('option', {name: 'Alpha'})).toBeTruthy();
      expect(screen.getByRole('option', {name: 'Beta'})).toBeTruthy();
    });

    it('reports the option value when one is picked', () => {
      const onInputChange = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} onInputChange={onInputChange} />);

      openList();
      fireEvent.click(screen.getByRole('option', {name: 'Beta'}));

      expect(onInputChange).toHaveBeenCalledWith('owner', 'beta');
    });

    it('accepts plain string options', () => {
      const onInputChange = vi.fn();
      const component = {...baseProps.component, options: ['one', 'two']};
      renderWithProviders(<SelectAdapter {...baseProps} component={component} onInputChange={onInputChange} />);

      openList();
      fireEvent.click(screen.getByRole('option', {name: 'two'}));

      expect(onInputChange).toHaveBeenCalledWith('owner', 'two');
    });

    it('shows the label of the current value', () => {
      renderWithProviders(<SelectAdapter {...baseProps} values={{owner: 'beta'}} />);

      expect(screen.getByRole('combobox').textContent).toBe('Beta');
    });

    it('keeps the placeholder row disabled when the field is required', () => {
      const component = {...baseProps.component, required: true};
      renderWithProviders(<SelectAdapter {...baseProps} component={component} />);

      openList();

      expect(screen.getByRole('option', {name: 'Select an owner'}).getAttribute('aria-disabled')).toBe('true');
    });

    it('lets an optional field be cleared through the placeholder row', () => {
      const onInputChange = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} values={{owner: 'alpha'}} onInputChange={onInputChange} />);

      openList();
      fireEvent.click(screen.getByRole('option', {name: 'Select an owner'}));

      expect(onInputChange).toHaveBeenCalledWith('owner', '');
    });

    it('disables the control while the form is loading', () => {
      renderWithProviders(<SelectAdapter {...baseProps} isLoading />);

      expect(screen.getByRole('combobox').getAttribute('aria-disabled')).toBe('true');
    });

    it('shows the field error only once the field is touched', () => {
      const fieldErrors = {owner: 'Owner is required'};
      const {rerender} = renderWithProviders(<SelectAdapter {...baseProps} fieldErrors={fieldErrors} />);
      expect(screen.queryByText('Owner is required')).toBeNull();

      rerender(<SelectAdapter {...baseProps} fieldErrors={fieldErrors} touched={{owner: true}} />);

      expect(screen.getByText('Owner is required')).toBeTruthy();
    });

    it('shows the hint', () => {
      const component = {...baseProps.component, hint: 'Pick the owner of the agent'};
      renderWithProviders(<SelectAdapter {...baseProps} component={component} />);

      expect(screen.getByText('Pick the owner of the agent')).toBeTruthy();
    });

    it('applies product prefix CSS class names', () => {
      renderWithProviders(<SelectAdapter {...baseProps} />);

      const formControl = document.querySelector(`.${TEST_CN_PREFIX}Flow--select`);
      expect(formControl).toBeTruthy();
      expect(formControl?.classList.contains(`${TEST_CN_PREFIX}FormControl--root`)).toBe(true);
    });

    it('returns null when ref is missing', () => {
      const component = {...baseProps.component, ref: undefined};
      const {container} = renderWithProviders(<SelectAdapter {...baseProps} component={component} />);

      expect(container.innerHTML).toBe('');
    });

    it('returns null when there are no options', () => {
      const component = {...baseProps.component, options: undefined};
      const {container} = renderWithProviders(<SelectAdapter {...baseProps} component={component} />);

      expect(container.innerHTML).toBe('');
    });

    it('shows no loading or empty row', () => {
      const component = {...baseProps.component, options: []};
      renderWithProviders(<SelectAdapter {...baseProps} component={component} />);

      openList();

      expect(screen.queryByText('status.loading')).toBeNull();
      expect(screen.queryByText('messages.noResults')).toBeNull();
    });
  });

  describe('options prop', () => {
    it('is used in place of the component options', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(2)} />);

      openList();

      expect(screen.getByRole('option', {name: 'User 1'})).toBeTruthy();
      expect(screen.queryByRole('option', {name: 'Alpha'})).toBeNull();
    });

    it('renders a SELECT without component options', () => {
      const component = {...baseProps.component, options: undefined};
      renderWithProviders(<SelectAdapter {...baseProps} component={component} options={optionsOf(1)} />);

      openList();

      expect(screen.getByRole('option', {name: 'User 1'})).toBeTruthy();
    });

    it('disables an option marked disabled', () => {
      const options = [{value: 'user-1', label: 'User 1', disabled: true}, ...optionsOf(1, 1)];
      renderWithProviders(<SelectAdapter {...baseProps} options={options} />);

      openList();

      expect(screen.getByRole('option', {name: 'User 1'}).getAttribute('aria-disabled')).toBe('true');
      expect(screen.getByRole('option', {name: 'User 2'}).getAttribute('aria-disabled')).toBeNull();
    });

    it('shows selectedOption for a value that is not among the options', () => {
      renderWithProviders(
        <SelectAdapter
          {...baseProps}
          options={optionsOf(2)}
          values={{owner: 'user-99'}}
          selectedOption={{value: 'user-99', label: 'Ninety Nine'}}
        />,
      );

      expect(screen.getByRole('combobox').textContent).toBe('Ninety Nine');
    });

    it('shows the raw value when nothing labels a value that is not among the options', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(2)} values={{owner: 'user-99'}} />);

      expect(screen.getByRole('combobox').textContent).toBe('user-99');
    });

    it('does not list the label of a missing value as a pickable option', () => {
      renderWithProviders(
        <SelectAdapter
          {...baseProps}
          options={optionsOf(2)}
          values={{owner: 'user-99'}}
          selectedOption={{value: 'user-99', label: 'Ninety Nine'}}
        />,
      );

      openList();

      expect(screen.queryByRole('option', {name: 'Ninety Nine'})).toBeNull();
    });

    it('lets the arrow keys move past the placeholder when the current value is not among the options', async () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(3)} values={{owner: 'user-99'}} />);
      openList();
      await settle();
      screen.getByRole('option', {name: 'Select an owner'}).focus();

      await userEvent.keyboard('{ArrowDown}');

      expect(document.activeElement?.textContent).toBe('User 1');
    });

    it('ignores selectedOption when it belongs to another value', () => {
      renderWithProviders(
        <SelectAdapter
          {...baseProps}
          options={optionsOf(2)}
          values={{owner: 'user-99'}}
          selectedOption={{value: 'user-7', label: 'Seven'}}
        />,
      );

      expect(screen.getByRole('combobox').textContent).toBe('user-99');
    });
  });

  describe('loading and empty rows', () => {
    it('shows a loading row while loading', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={[]} loading onLoadMore={vi.fn()} />);

      openList();

      expect(screen.getByText('status.loading')).toBeTruthy();
    });

    it('shows the empty row when the list is empty and nothing more can load', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={[]} />);

      openList();

      expect(screen.getByText('messages.noResults')).toBeTruthy();
    });

    it('shows emptyText in the empty row', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={[]} emptyText="No users yet" />);

      openList();

      expect(screen.getByText('No users yet')).toBeTruthy();
    });

    it('does not show the empty row while more can load', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={[]} onLoadMore={vi.fn()} />);

      openList();

      expect(screen.queryByText('messages.noResults')).toBeNull();
    });

    it('does not show the empty row while loading', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={[]} loading />);

      openList();

      expect(screen.queryByText('messages.noResults')).toBeNull();
    });

    it('does not show the empty row when there are options', () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(1)} />);

      openList();

      expect(screen.queryByText('messages.noResults')).toBeNull();
    });
  });

  describe('onOpen', () => {
    it('is called when the menu opens', () => {
      const onOpen = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} onOpen={onOpen} />);

      openList();

      expect(onOpen).toHaveBeenCalledTimes(1);
    });

    it('is not called while the menu stays closed', () => {
      const onOpen = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} onOpen={onOpen} />);

      expect(onOpen).not.toHaveBeenCalled();
    });

    it('is called once per open', async () => {
      const onOpen = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} onOpen={onOpen} />);

      openList();
      fireEvent.keyDown(screen.getByRole('listbox'), {key: 'Escape'});
      await waitFor(() => expect(screen.queryByRole('listbox')).toBeNull());
      openList();

      expect(onOpen).toHaveBeenCalledTimes(2);
    });
  });

  describe('loading more', () => {
    it('asks for more when the end of a short list is visible', async () => {
      const onLoadMore = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} />);

      openList();

      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(1));
    });

    it('does not ask while the menu is closed', async () => {
      const onLoadMore = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} />);

      await settle();

      expect(onLoadMore).not.toHaveBeenCalled();
    });

    it('does not ask when no onLoadMore is given', async () => {
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(3)} />);

      openList();
      await settle();

      expect(screen.getAllByRole('option').length).toBeGreaterThan(0);
    });

    it('does not ask while loading', async () => {
      const onLoadMore = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(3)} loading onLoadMore={onLoadMore} />);

      openList();
      await settle();

      expect(onLoadMore).not.toHaveBeenCalled();
    });

    it('asks once loading has finished', async () => {
      const onLoadMore = vi.fn();
      const {rerender} = renderWithProviders(
        <SelectAdapter {...baseProps} options={optionsOf(3)} loading onLoadMore={onLoadMore} />,
      );
      openList();
      await settle();

      rerender(<SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} />);

      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(1));
    });

    it('asks only once for the same list length', async () => {
      const onLoadMore = vi.fn();
      const {rerender} = renderWithProviders(
        <SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} />,
      );
      openList();
      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(1));

      rerender(<SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} values={{owner: ''}} />);
      await settle();

      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('asks again after the list grew', async () => {
      const onLoadMore = vi.fn();
      const {rerender} = renderWithProviders(
        <SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} />,
      );
      openList();
      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(1));

      rerender(<SelectAdapter {...baseProps} options={optionsOf(6)} onLoadMore={onLoadMore} />);

      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(2));
    });

    it('stops asking once onLoadMore is removed', async () => {
      const onLoadMore = vi.fn();
      const {rerender} = renderWithProviders(
        <SelectAdapter {...baseProps} options={optionsOf(3)} onLoadMore={onLoadMore} />,
      );
      openList();
      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(1));

      rerender(<SelectAdapter {...baseProps} options={optionsOf(6)} />);
      await settle();

      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('waits for the end of a long list to be scrolled into view', async () => {
      const onLoadMore = vi.fn();
      renderWithProviders(<SelectAdapter {...baseProps} options={optionsOf(100)} onLoadMore={onLoadMore} />);

      openList();
      await settle();
      expect(onLoadMore).not.toHaveBeenCalled();

      const menuPaper = screen.getByRole('listbox').parentElement!;
      menuPaper.scrollTop = menuPaper.scrollHeight;

      await waitFor(() => expect(onLoadMore).toHaveBeenCalledTimes(1));
    });
  });
});
