// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, cleanup, fireEvent, screen, waitFor} from '@testing-library/react';
import {afterEach, describe, it, expect, vi} from 'vitest';
import {pageOf, pagedLoader, type FetchPage, type PagedPage} from '../../../../test/pagedOptions';
import renderWithProviders from '../../../../test/renderWithProviders';
import {openList, settle} from '../../../../test/selectInteractions';
import ResourceSelectAdapter, {type ResourceSelectAdapterProps} from '../ResourceSelectAdapter';

afterEach(() => {
  cleanup();
});

const baseProps = (fetchPage: FetchPage): ResourceSelectAdapterProps & {fetchPage: FetchPage} => ({
  component: {
    id: 'owner_input',
    type: 'USER_SELECT',
    ref: 'owner',
    label: 'Owner',
    placeholder: 'Select an owner',
    required: false,
  },
  fetchPage,
  isLoading: false,
  onInputChange: vi.fn(),
  resolve: (s) => s,
  values: {},
});

const scrollListToEnd = (): void => {
  const menuPaper = screen.getByRole('listbox').parentElement!;
  menuPaper.scrollTop = menuPaper.scrollHeight;
};

describe('ResourceSelectAdapter', () => {
  describe('first page', () => {
    it('renders the label and the placeholder without requesting anything', async () => {
      const fetchPage = pagedLoader(50);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);

      await settle();

      expect(screen.getByText('Owner')).toBeTruthy();
      expect(screen.getByText('Select an owner')).toBeTruthy();
      expect(fetchPage).not.toHaveBeenCalled();
    });

    it('loads the first page when the list opens', async () => {
      const fetchPage = pagedLoader(50);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);

      openList();

      expect(await screen.findByRole('option', {name: 'User 1'})).toBeTruthy();
      expect(fetchPage).toHaveBeenCalledTimes(1);
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: 30, offset: 0}));
    });

    it('shows a loading row while the first page is in flight', async () => {
      let resolvePage: (page: PagedPage) => void = () => undefined;
      const fetchPage = vi.fn<FetchPage>(
        () =>
          new Promise<PagedPage>((resolve) => {
            resolvePage = resolve;
          }),
      );
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);

      openList();

      expect(screen.getByText('status.loading')).toBeTruthy();
      act(() => resolvePage(pageOf(0, 30, 3)));
      await waitFor(() => expect(screen.queryByText('status.loading')).toBeNull());
    });

    it('reports the id of the option that is picked', async () => {
      const onInputChange = vi.fn();
      renderWithProviders(<ResourceSelectAdapter {...baseProps(pagedLoader(3))} onInputChange={onInputChange} />);

      openList();
      fireEvent.click(await screen.findByRole('option', {name: 'User 2'}));

      expect(onInputChange).toHaveBeenCalledWith('owner', 'user-2');
    });

    it('shows the empty row when there is nothing to pick', async () => {
      renderWithProviders(<ResourceSelectAdapter {...baseProps(pagedLoader(0))} />);

      openList();

      expect(await screen.findByText('messages.noResults')).toBeTruthy();
    });

    it('does not request again when the list is reopened', async () => {
      const fetchPage = pagedLoader(3);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);
      openList();
      await screen.findByRole('option', {name: 'User 1'});
      fireEvent.keyDown(screen.getByRole('listbox'), {key: 'Escape'});
      await waitFor(() => expect(screen.queryByRole('listbox')).toBeNull());

      openList();
      await settle();

      expect(fetchPage).toHaveBeenCalledTimes(1);
      expect(screen.getByRole('option', {name: 'User 3'})).toBeTruthy();
    });

    it('is disabled while the form is loading', () => {
      renderWithProviders(<ResourceSelectAdapter {...baseProps(pagedLoader(3))} isLoading />);

      expect(screen.getByRole('combobox').getAttribute('aria-disabled')).toBe('true');
    });
  });

  describe('more pages', () => {
    it('loads the next page when the end of the list is scrolled into view', async () => {
      const fetchPage = pagedLoader(50);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);
      openList();
      await screen.findByRole('option', {name: 'User 30'});
      await settle();
      expect(fetchPage).toHaveBeenCalledTimes(1);

      scrollListToEnd();

      expect(await screen.findByRole('option', {name: 'User 50'})).toBeTruthy();
      expect(fetchPage).toHaveBeenCalledTimes(2);
      expect(fetchPage).toHaveBeenLastCalledWith(expect.objectContaining({offset: 30}));
    });

    it('stops requesting once the last page has loaded', async () => {
      const fetchPage = pagedLoader(40);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);
      openList();
      await screen.findByRole('option', {name: 'User 30'});
      await settle();
      scrollListToEnd();
      await screen.findByRole('option', {name: 'User 40'});

      scrollListToEnd();
      await settle();

      expect(fetchPage).toHaveBeenCalledTimes(2);
    });

    it('requests the next page of a short first page without scrolling', async () => {
      const fetchPage = vi.fn<FetchPage>(({offset}) => Promise.resolve(pageOf(offset, 5, 12)));
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} />);

      openList();

      expect(await screen.findByRole('option', {name: 'User 12'})).toBeTruthy();
      expect(fetchPage).toHaveBeenCalledTimes(3);
    });

    it('uses the given page size', async () => {
      const fetchPage = pagedLoader(50);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} pageSize={10} />);

      openList();

      await screen.findByRole('option', {name: 'User 10'});
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: 10}));
    });

    it('sends the filter with each request', async () => {
      const fetchPage = pagedLoader(3);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} filter="alice" />);

      openList();

      await screen.findByRole('option', {name: 'User 1'});
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({filter: 'alice'}));
    });
  });

  describe('failures', () => {
    it('reports a failed load and offers nothing to pick', async () => {
      const failure = new Error('forbidden');
      const onError = vi.fn();
      const fetchPage = vi.fn<FetchPage>().mockRejectedValue(failure);
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} onError={onError} />);

      openList();

      await waitFor(() => expect(onError).toHaveBeenCalledWith(failure));
      expect(screen.getAllByRole('option')).toHaveLength(1);
    });

    it('loads the first page again when the list is reopened after a failure', async () => {
      const fetchPage = vi
        .fn<FetchPage>()
        .mockRejectedValueOnce(new Error('unavailable'))
        .mockImplementation(({limit, offset}) => Promise.resolve(pageOf(offset, limit, 3)));
      const onError = vi.fn();
      renderWithProviders(<ResourceSelectAdapter {...baseProps(fetchPage)} onError={onError} />);
      openList();
      await waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
      fireEvent.keyDown(screen.getByRole('listbox'), {key: 'Escape'});
      await waitFor(() => expect(screen.queryByRole('listbox')).toBeNull());

      openList();

      expect(await screen.findByRole('option', {name: 'User 1'})).toBeTruthy();
    });
  });

  describe('default value', () => {
    it('selects the default value when the field is empty', () => {
      const onInputChange = vi.fn();
      renderWithProviders(
        <ResourceSelectAdapter {...baseProps(pagedLoader(3))} defaultValue="user-2" onInputChange={onInputChange} />,
      );

      expect(onInputChange).toHaveBeenCalledTimes(1);
      expect(onInputChange).toHaveBeenCalledWith('owner', 'user-2');
    });

    it('does not overwrite a value that is already set', () => {
      const onInputChange = vi.fn();
      renderWithProviders(
        <ResourceSelectAdapter
          {...baseProps(pagedLoader(3))}
          defaultValue="user-2"
          values={{owner: 'user-3'}}
          onInputChange={onInputChange}
        />,
      );

      expect(onInputChange).not.toHaveBeenCalled();
    });

    it('selects the default value once it becomes known', () => {
      const onInputChange = vi.fn();
      const props = {...baseProps(pagedLoader(3)), onInputChange};
      const {rerender} = renderWithProviders(<ResourceSelectAdapter {...props} />);
      expect(onInputChange).not.toHaveBeenCalled();

      rerender(<ResourceSelectAdapter {...props} defaultValue="user-1" />);

      expect(onInputChange).toHaveBeenCalledWith('owner', 'user-1');
    });

    it('selects it again once the form values were reset', () => {
      const onInputChange = vi.fn();
      const props = {...baseProps(pagedLoader(3)), defaultValue: 'user-2', onInputChange};
      const {rerender} = renderWithProviders(<ResourceSelectAdapter {...props} />);
      rerender(<ResourceSelectAdapter {...props} values={{owner: 'user-2'}} />);

      rerender(<ResourceSelectAdapter {...props} values={{}} />);

      expect(onInputChange).toHaveBeenCalledTimes(2);
    });

    it('does not select it again after the field was cleared', () => {
      const onInputChange = vi.fn();
      const props = {...baseProps(pagedLoader(3)), defaultValue: 'user-2', onInputChange};
      const {rerender} = renderWithProviders(<ResourceSelectAdapter {...props} />);
      rerender(<ResourceSelectAdapter {...props} values={{owner: 'user-2'}} />);

      rerender(<ResourceSelectAdapter {...props} values={{owner: ''}} />);

      expect(onInputChange).toHaveBeenCalledTimes(1);
    });

    it('shows the default label before the page that holds the value has loaded', () => {
      renderWithProviders(
        <ResourceSelectAdapter
          {...baseProps(pagedLoader(50))}
          defaultValue="user-45"
          defaultLabel="Dana Owner"
          values={{owner: 'user-45'}}
        />,
      );

      expect(screen.getByRole('combobox').textContent).toBe('Dana Owner');
    });

    it('prefers the loaded option over the default label', async () => {
      renderWithProviders(
        <ResourceSelectAdapter
          {...baseProps(pagedLoader(3))}
          defaultValue="user-2"
          defaultLabel="Dana Owner"
          values={{owner: 'user-2'}}
        />,
      );
      openList();
      await screen.findByRole('option', {name: 'User 2'});

      expect(screen.getByRole('combobox', {hidden: true}).textContent).toBe('User 2');
    });
  });
});
