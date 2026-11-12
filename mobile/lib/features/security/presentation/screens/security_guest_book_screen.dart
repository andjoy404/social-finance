import 'dart:async';
import 'package:flutter/material.dart';
import '../utils/guest_visit_duration.dart' as guest_duration;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import '../../data/security_models.dart';
import '../providers/security_providers.dart';

class SecurityGuestBookScreen extends ConsumerStatefulWidget {
  const SecurityGuestBookScreen({super.key});
  @override
  ConsumerState<SecurityGuestBookScreen> createState() =>
      _SecurityGuestBookScreenState();
}

class _SecurityGuestBookScreenState
    extends ConsumerState<SecurityGuestBookScreen>
    with WidgetsBindingObserver {
  final _search = TextEditingController();
  Timer? _debounce;
  Timer? _guestTicker;
  bool _loading = true;
  String? _error;
  List<GuestBookEntry> _entries = [];
  int _page = 1, _total = 0, _totalPages = 1;
  String _status = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _ensureGuestTicker();
    } else if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.inactive ||
        state == AppLifecycleState.detached ||
        state == AppLifecycleState.hidden) {
      _guestTicker?.cancel();
      _guestTicker = null;
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _debounce?.cancel();
    _guestTicker?.cancel();
    _search.dispose();
    super.dispose();
  }

  Future<void> _load({int? page}) async {
    final targetPage = page ?? _page;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final result = await ref
          .read(securityApiClientProvider)
          .listManagerGuestBooks(
            search: _search.text,
            status: _status,
            page: targetPage,
          );
      if (!mounted) {
        return;
      }
      setState(() {
        _entries = result.data;
        _total = result.total;
        _totalPages = result.totalPages;
        _page = targetPage;
        _loading = false;
      });
      _ensureGuestTicker();
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  void _ensureGuestTicker() {
    final hasActive = _entries.any((g) => g.checkedOutAt == null);
    if (hasActive && _guestTicker == null) {
      _guestTicker = Timer.periodic(const Duration(seconds: 30), (_) {
        if (!mounted) return;
        setState(() {});
      });
    } else if (!hasActive) {
      _guestTicker?.cancel();
      _guestTicker = null;
    }
  }

  String _guestDurationLabel(GuestBookEntry g) {
    return guest_duration.guestVisitDurationLabel(
      g.checkedInAt,
      g.checkedOutAt,
    );
  }

  Future<void> _detail(GuestBookEntry g) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setD) => AlertDialog(
          title: Text(g.guestName),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _line('Tujuan', g.destinationLabel),
                _line('Keperluan', g.purpose),
                _line(
                  'Masuk',
                  DateFormat(
                    'dd/MM/yyyy HH:mm',
                  ).format(g.checkedInAt.toLocal()),
                ),
                _line(
                  'Keluar',
                  g.checkedOutAt == null
                      ? 'Belum keluar'
                      : DateFormat(
                          'dd/MM/yyyy HH:mm',
                        ).format(g.checkedOutAt!.toLocal()),
                ),
                _line(
                  g.checkedOutAt == null ? 'Status' : 'Durasi',
                  _guestDurationLabel(g),
                ),
                _line('Telepon', g.phoneMasked ?? '-'),
                _line('Perangkat', g.deviceName),
                _line('Catatan', g.notes ?? '-'),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Tutup'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _line(String label, String value) => Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Text('$label: $value'),
  );

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: const Text('Buku Tamu'),
      actions: [
        IconButton(
          onPressed: () => _load(page: 1),
          icon: const Icon(Icons.refresh),
        ),
      ],
    ),
    body: Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            children: [
              TextField(
                controller: _search,
                onChanged: (value) {
                  _debounce?.cancel();
                  _debounce = Timer(
                    const Duration(milliseconds: 350),
                    () => _load(page: 1),
                  );
                },
                decoration: InputDecoration(
                  hintText: 'Cari nama atau tujuan kunjungan...',
                  prefixIcon: const Icon(Icons.search),
                  suffixIcon: _search.text.isEmpty
                      ? null
                      : IconButton(
                          icon: const Icon(Icons.clear),
                          onPressed: () {
                            _search.clear();
                            _load(page: 1);
                          },
                        ),
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                value: _status,
                decoration: const InputDecoration(
                  labelText: 'Status kunjungan',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
                items: const [
                  DropdownMenuItem(value: '', child: Text('Semua')),
                  DropdownMenuItem(value: 'active', child: Text('Masih di RT')),
                  DropdownMenuItem(
                    value: 'checked_out',
                    child: Text('Sudah Keluar'),
                  ),
                ],
                onChanged: (v) {
                  setState(() => _status = v ?? '');
                  _load(page: 1);
                },
              ),
            ],
          ),
        ),
        Expanded(
          child: _loading
              ? const Center(child: CircularProgressIndicator())
              : _error != null
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(_error!, textAlign: TextAlign.center),
                      const SizedBox(height: 12),
                      FilledButton(
                        onPressed: () => _load(),
                        child: const Text('Coba Lagi'),
                      ),
                    ],
                  ),
                )
              : _entries.isEmpty
              ? const Center(child: Text('Tidak ada catatan Buku Tamu.'))
              : RefreshIndicator(
                  onRefresh: () => _load(),
                  child: ListView.builder(
                    itemCount: _entries.length,
                    itemBuilder: (ctx, i) {
                      final g = _entries[i];
                      return Card(
                        margin: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 5,
                        ),
                        child: ListTile(
                          title: Text('${g.guestName} → ${g.destinationLabel}'),
                          subtitle: Text(
                            '${g.purpose}\nMasuk ${DateFormat('dd/MM HH:mm').format(g.checkedInAt)} · ${g.checkedOutAt == null ? 'Belum keluar' : 'Keluar ${DateFormat('HH:mm').format(g.checkedOutAt!)}'}\n${_guestDurationLabel(g)}',
                          ),
                          trailing: const Icon(Icons.chevron_right),
                          onTap: () => _detail(g),
                        ),
                      );
                    },
                  ),
                ),
        ),
        if (_totalPages > 1)
          SafeArea(
            top: false,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                IconButton(
                  onPressed: _page > 1 ? () => _load(page: _page - 1) : null,
                  icon: const Icon(Icons.chevron_left),
                ),
                Text('Halaman $_page / $_totalPages · $_total'),
                IconButton(
                  onPressed: _page < _totalPages
                      ? () => _load(page: _page + 1)
                      : null,
                  icon: const Icon(Icons.chevron_right),
                ),
              ],
            ),
          ),
      ],
    ),
  );
}
