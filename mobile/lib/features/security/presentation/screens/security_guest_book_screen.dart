import 'dart:async';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import '../utils/guest_visit_duration.dart' as guest_duration;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import '../../data/security_models.dart';
import '../providers/security_providers.dart';
import '../utils/attendance_order.dart';
import '../widgets/guest_visit_status_badge.dart';

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
  bool _refreshing = false;
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

  Future<void> _guardedRefresh() async {
    if (_refreshing) return;
    _refreshing = true;
    try {
      await _load(isRefresh: true);
    } finally {
      _refreshing = false;
    }
  }

  Future<void> _load({int? page, bool isRefresh = false}) async {
    final targetPage = page ?? _page;
    if (!isRefresh) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
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
      final sorted = [...result.data]..sort(compareGuestBookChronological);
      setState(() {
        _entries = sorted;
        _total = result.total;
        _totalPages = result.totalPages;
        _page = targetPage;
        _loading = false;
      });
      _ensureGuestTicker();
    } catch (e) {
      if (mounted) {
        if (isRefresh) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('Gagal memuat ulang: $e')));
        } else {
          setState(() {
            _error = e.toString();
            _loading = false;
          });
        }
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
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: GuestVisitStatusBadge(checkedOutAt: g.checkedOutAt),
                ),
                if (g.photoKey != null && g.photoKey!.isNotEmpty) ...[
                  GestureDetector(
                    onTap: () => _showGuestBookPhotoPreview(context, g.photoKey!),
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(8),
                      child: Container(
                        width: double.infinity,
                        height: 160,
                        color: Colors.grey[200],
                        child: _buildGuestBookImage(g.photoKey!),
                      ),
                    ),
                  ),
                  const SizedBox(height: 8),
                ],
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
                if (g.notes != null && g.notes!.isNotEmpty)
                  _line('Catatan', g.notes!),
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

  Widget _buildGuestBookImage(String photoKey) {
    final apiClient = ref.read(securityApiClientProvider);
    return FutureBuilder<Uint8List?>(
      future: apiClient.fetchGuestBookPhoto(photoKey),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator.adaptive());
        }
        if (snapshot.hasError || snapshot.data == null || snapshot.data!.isEmpty) {
          return Center(
            child: Icon(Icons.broken_image, size: 40, color: Colors.grey[600]),
          );
        }
        return Image.memory(
          snapshot.data!,
          fit: BoxFit.cover,
          errorBuilder: (context, error, stackTrace) {
            return Center(
              child: Icon(Icons.broken_image, size: 40, color: Colors.grey[600]),
            );
          },
        );
      },
    );
  }

  void _showGuestBookPhotoPreview(BuildContext context, String photoKey) {
    final apiClient = ref.read(securityApiClientProvider);
    showDialog(
      context: context,
      builder: (ctx) => Dialog(
        backgroundColor: Colors.black,
        insetPadding: const EdgeInsets.all(12),
        child: Stack(
          alignment: Alignment.topRight,
          children: [
            Center(
              child: InteractiveViewer(
                child: FutureBuilder<Uint8List?>(
                  future: apiClient.fetchGuestBookPhoto(photoKey),
                  builder: (context, snapshot) {
                    if (snapshot.connectionState == ConnectionState.waiting) {
                      return const Center(
                        child: CircularProgressIndicator.adaptive(
                          backgroundColor: Colors.white,
                        ),
                      );
                    }
                    if (snapshot.hasError || snapshot.data == null || snapshot.data!.isEmpty) {
                      return Center(
                        child: Icon(
                          Icons.broken_image,
                          size: 60,
                          color: Colors.white70,
                        ),
                      );
                    }
                    return Image.memory(
                      snapshot.data!,
                      fit: BoxFit.contain,
                    );
                  },
                ),
              ),
            ),
            IconButton(
              icon: const Icon(Icons.close, color: Colors.white, size: 28),
              onPressed: () => Navigator.of(ctx).pop(),
            ),
          ],
        ),
      ),
    );
  }

  void _showPhotoPreview(BuildContext context, String filePath) {
    showDialog(
      context: context,
      builder: (ctx) => Dialog(
        backgroundColor: Colors.black,
        insetPadding: const EdgeInsets.all(12),
        child: Stack(
          alignment: Alignment.topRight,
          children: [
            Center(
              child: InteractiveViewer(
                child: Image.file(File(filePath), fit: BoxFit.contain),
              ),
            ),
            IconButton(
              icon: const Icon(Icons.close, color: Colors.white, size: 28),
              onPressed: () => Navigator.of(ctx).pop(),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _openCreateGuestDialog() async {
    final s = ref.read(posSessionProvider).session;
    if (s == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Perangkat belum dipasangkan sebagai POS. Silakan hubungkan perangkat terlebih dahulu.',
          ),
        ),
      );
      return;
    }
    final nameCtrl = TextEditingController();
    final phoneCtrl = TextEditingController();
    final purposeCtrl = TextEditingController();
    final destinationCtrl = TextEditingController();
    final plateCtrl = TextEditingController();
    final notesCtrl = TextEditingController();
    XFile? capturedPhoto;
    bool isProcessingCamera = false;

    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final theme = Theme.of(ctx);
        return StatefulBuilder(
          builder: (ctx, setDialogState) => AlertDialog(
            title: const Text('Catat Tamu Masuk'),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(
                    controller: nameCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Nama Tamu *',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: phoneCtrl,
                    decoration: const InputDecoration(
                      labelText: 'No. HP (Opsional)',
                      border: OutlineInputBorder(),
                    ),
                    keyboardType: TextInputType.phone,
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: destinationCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Rumah / Tujuan *',
                      hintText: 'Contoh: Rumah No. 12 Pak Budi',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: purposeCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Keperluan *',
                      hintText: 'Contoh: Mengantar Paket / Bertamu',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: plateCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Plat Kendaraan (Opsional)',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: notesCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Catatan (Opsional)',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 14),
                  Text(
                    'Foto Tamu (Kamera Lapangan)',
                    style: theme.textTheme.labelMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 8),
                  if (capturedPhoto != null) ...[
                    Row(
                      children: [
                        GestureDetector(
                          onTap: () =>
                              _showPhotoPreview(ctx, capturedPhoto!.path),
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: Container(
                              width: 80,
                              height: 80,
                              decoration: BoxDecoration(
                                border: Border.all(
                                  color: AppColors.accent,
                                  width: 2,
                                ),
                                borderRadius: BorderRadius.circular(8),
                              ),
                              child: Image.file(
                                File(capturedPhoto!.path),
                                width: 80,
                                height: 80,
                                fit: BoxFit.cover,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              OutlinedButton.icon(
                                icon: const Icon(Icons.refresh, size: 16),
                                label: const Text(
                                  'Ambil Ulang',
                                  style: TextStyle(fontSize: 12),
                                ),
                                onPressed: isProcessingCamera
                                    ? null
                                    : () async {
                                        setDialogState(
                                          () => isProcessingCamera = true,
                                        );
                                        try {
                                          final picker = ImagePicker();
                                          final photo = await picker.pickImage(
                                            source: ImageSource.camera,
                                            imageQuality: 85,
                                            maxWidth: 1280,
                                          );
                                          if (photo != null) {
                                            setDialogState(
                                              () => capturedPhoto = photo,
                                            );
                                          }
                                        } catch (e) {
                                          if (mounted) {
                                            ScaffoldMessenger.of(
                                              context,
                                            ).showSnackBar(
                                              SnackBar(
                                                content: Text(
                                                  'Gagal mengakses kamera: $e',
                                                ),
                                              ),
                                            );
                                          }
                                        } finally {
                                          setDialogState(
                                            () => isProcessingCamera = false,
                                          );
                                        }
                                      },
                              ),
                              const SizedBox(height: 4),
                              TextButton.icon(
                                icon: const Icon(
                                  Icons.delete_outline,
                                  size: 16,
                                  color: AppColors.danger,
                                ),
                                label: const Text(
                                  'Hapus Foto',
                                  style: TextStyle(
                                    fontSize: 12,
                                    color: AppColors.danger,
                                  ),
                                ),
                                onPressed: isProcessingCamera
                                    ? null
                                    : () {
                                        setDialogState(
                                          () => capturedPhoto = null,
                                        );
                                      },
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ] else ...[
                    OutlinedButton.icon(
                      icon: isProcessingCamera
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.camera_alt_outlined),
                      label: const Text('Ambil Foto Tamu (Kamera)'),
                      onPressed: isProcessingCamera
                          ? null
                          : () async {
                              setDialogState(() => isProcessingCamera = true);
                              try {
                                final picker = ImagePicker();
                                final photo = await picker.pickImage(
                                  source: ImageSource.camera,
                                  imageQuality: 85,
                                  maxWidth: 1280,
                                );
                                if (photo != null) {
                                  setDialogState(() => capturedPhoto = photo);
                                }
                              } catch (e) {
                                if (mounted) {
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(
                                        'Gagal mengakses kamera: $e',
                                      ),
                                    ),
                                  );
                                }
                              } finally {
                                setDialogState(
                                  () => isProcessingCamera = false,
                                );
                              }
                            },
                    ),
                  ],
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(ctx).pop(false),
                child: const Text('Batal'),
              ),
              ElevatedButton(
                onPressed: isProcessingCamera
                    ? null
                    : () => Navigator.of(ctx).pop(true),
                child: const Text('Simpan Tamu'),
              ),
            ],
          ),
        );
      },
    );
    if (ok != true || !mounted) return;
    if (nameCtrl.text.trim().isEmpty ||
        destinationCtrl.text.trim().isEmpty ||
        purposeCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Nama tamu, tujuan, dan keperluan wajib diisi.'),
        ),
      );
      return;
    }

    final combinedNotes = capturedPhoto != null
        ? (notesCtrl.text.trim().isNotEmpty
              ? notesCtrl.text.trim()
              : null)
        : (notesCtrl.text.trim().isNotEmpty ? notesCtrl.text.trim() : null);

    String? photoKey;
    if (capturedPhoto != null) {
      try {
        final photoBytes = await capturedPhoto!.readAsBytes();
        photoKey = await ref
            .read(securityApiClientProvider)
            .uploadGuestBookPhoto(photoBytes, capturedPhoto!.name);
      } catch (e) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal mengunggah foto: $e')),
        );
        return;
      }
    }

    try {
      await ref
          .read(securityApiClientProvider)
          .createPosGuestBook(
            s,
            guestName: nameCtrl.text,
            phone: phoneCtrl.text,
            purpose: purposeCtrl.text,
            destinationLabel: destinationCtrl.text,
            vehiclePlate: plateCtrl.text,
            notes: combinedNotes,
            photoKey: photoKey,
          );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Data tamu berhasil dicatat.')),
      );
      await _load(page: 1);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Widget _line(String label, String value) => Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Text('$label: $value'),
  );

  @override
  Widget build(BuildContext context) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final role = user?['role']?.toString().toLowerCase() ?? '';
    final title = (user?['app_title'] ?? user?['jabatan'] ?? '')
        .toString()
        .toLowerCase();
    final isOfficer =
        role == 'keamanan' ||
        title.contains('keamanan') ||
        title.contains('satpam');

    return Scaffold(
      appBar: AppBar(
        title: const Text('Buku Tamu'),
        actions: [
          IconButton(
            onPressed: () => _load(page: 1),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      floatingActionButton: isOfficer
          ? FloatingActionButton.extended(
              onPressed: _openCreateGuestDialog,
              backgroundColor: AppColors.accent,
              foregroundColor: Colors.white,
              icon: const Icon(Icons.person_add_alt_1),
              label: const Text('Catat Tamu'),
            )
          : null,
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
                    DropdownMenuItem(
                      value: 'active',
                      child: Text('Masih di RT'),
                    ),
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
                ? ListView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    children: const [
                      SizedBox(height: 220),
                      Center(child: CircularProgressIndicator()),
                    ],
                  )
                : _error != null
                ? RefreshIndicator(
                    onRefresh: _guardedRefresh,
                    child: ListView(
                      physics: const AlwaysScrollableScrollPhysics(),
                      children: [
                        const SizedBox(height: 220),
                        Center(
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
                        ),
                      ],
                    ),
                  )
                : _entries.isEmpty
                ? RefreshIndicator(
                    onRefresh: _guardedRefresh,
                    child: ListView(
                      physics: const AlwaysScrollableScrollPhysics(),
                      children: const [
                        SizedBox(height: 220),
                        Center(child: Text('Tidak ada catatan Buku Tamu.')),
                      ],
                    ),
                  )
                : RefreshIndicator(
                    onRefresh: _guardedRefresh,
                    child: ListView.builder(
                      physics: const AlwaysScrollableScrollPhysics(),
                      itemCount: _entries.length,
                      itemBuilder: (ctx, i) {
                        final g = _entries[i];
                        return Card(
                          margin: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 5,
                          ),
                          child: ListTile(
                            title: Text(
                              '${g.guestName} → ${g.destinationLabel}',
                            ),
                            subtitle: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                GuestVisitStatusBadge(
                                  checkedOutAt: g.checkedOutAt,
                                ),
                                const SizedBox(height: 4),
                                Text(g.purpose),
                                const SizedBox(height: 2),
                                Text(
                                  'Masuk: ${DateFormat('dd/MM HH:mm').format(g.checkedInAt.toLocal())}',
                                ),
                                const SizedBox(height: 2),
                                Text(
                                  g.checkedOutAt == null
                                      ? 'Belum keluar'
                                      : 'Keluar: ${DateFormat('dd/MM HH:mm').format(g.checkedOutAt!.toLocal())}',
                                ),
                                const SizedBox(height: 2),
                                Text(_guestDurationLabel(g)),
                              ],
                            ),
                            isThreeLine: true,
                            trailing: const Icon(Icons.chevron_right),
                            onTap: () => _detail(g),
                          ),
                        );
                      },
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}
