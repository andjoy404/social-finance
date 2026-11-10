import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

const _allowedDeepLinkPrefixes = [
  '/keuangan',
  '/iuran',
  '/kas',
  '/kegiatan',
  '/warga',
  '/notifikasi',
  '/pengumuman',
  '/profil',
];

class AnnouncementFormScreen extends ConsumerStatefulWidget {
  final String? announcementId;

  const AnnouncementFormScreen({super.key, this.announcementId});

  @override
  ConsumerState<AnnouncementFormScreen> createState() =>
      _AnnouncementFormScreenState();
}

class _AnnouncementFormScreenState
    extends ConsumerState<AnnouncementFormScreen> {
  final _titleCtrl = TextEditingController();
  final _bodyCtrl = TextEditingController();
  final _deepLinkCtrl = TextEditingController();
  final _priorityCtrl = TextEditingController(text: '10');
  final _displayOrderCtrl = TextEditingController(text: '0');

  List<BannerTemplate> _templates = [];
  String? _selectedTemplateId;
  String? _imageUrl;
  DateTime? _scheduledStart;
  DateTime? _scheduledEnd;
  bool _publishNow = true;

  bool _isLoading = false;
  bool _isSaving = false;
  bool _isUploadingImage = false;
  String? _error;

  bool get _isEditing =>
      widget.announcementId != null && widget.announcementId!.isNotEmpty;

  @override
  void initState() {
    super.initState();
    if (Platform.environment.containsKey('FLUTTER_TEST')) {
      return;
    }
    _bootstrap();
  }

  Future<void> _bootstrap() async {
    setState(() => _isLoading = true);
    final repo = ref.read(communityRepositoryProvider);
    try {
      final templates = await repo.listBannerTemplates();
      if (!mounted) return;

      CommunityAnnouncement? existing;
      if (_isEditing) {
        existing = await repo.getAnnouncement(widget.announcementId!);
      }
      if (!mounted) return;

      setState(() {
        _templates = templates;
        if (existing != null) {
          _titleCtrl.text = existing.title;
          _bodyCtrl.text = existing.body;
          _deepLinkCtrl.text = existing.deepLink ?? '';
          _priorityCtrl.text = existing.priority.toString();
          _displayOrderCtrl.text = existing.displayOrder.toString();
          _scheduledStart = existing.scheduledStart?.toLocal();
          _scheduledEnd = existing.scheduledEnd?.toLocal();
          _imageUrl = existing.imageUrl;
          _selectedTemplateId = existing.templateId;
          _publishNow = false;
        }
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  @override
  void dispose() {
    _titleCtrl.dispose();
    _bodyCtrl.dispose();
    _deepLinkCtrl.dispose();
    _priorityCtrl.dispose();
    _displayOrderCtrl.dispose();
    super.dispose();
  }

  Future<DateTime?> _pickDateTime(DateTime? initial) async {
    final now = DateTime.now();
    final base = initial ?? now.add(const Duration(days: 1));
    final date = await showDatePicker(
      context: context,
      initialDate: base,
      firstDate: DateTime(now.year - 1),
      lastDate: DateTime(now.year + 5),
    );
    if (date == null || !mounted) return null;
    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay(hour: base.hour, minute: base.minute),
    );
    if (time == null) return null;
    return DateTime(date.year, date.month, date.day, time.hour, time.minute);
  }

  Future<void> _pickImage() async {
    final picker = ImagePicker();
    final picked = await picker.pickImage(
      source: ImageSource.gallery,
      imageQuality: 85,
      maxWidth: 1600,
    );
    if (picked == null || !mounted) return;

    final file = File(picked.path);
    final size = await file.length();
    if (size > 5 * 1024 * 1024) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Ukuran gambar maksimal 5 MB')),
        );
      }
      return;
    }

    setState(() {
      _isUploadingImage = true;
      _error = null;
    });
    try {
      final repo = ref.read(communityRepositoryProvider);
      final res = await repo.uploadCommunityImage(
        file,
        'announcement',
      );
      final url = res['image_url'];
      if (url is String && url.isNotEmpty) {
        setState(() => _imageUrl = url);
      } else {
        setState(() => _error = 'Gagal mengunggah gambar');
      }
    } catch (_) {
      setState(() => _error = 'Gagal mengunggah gambar');
    } finally {
      if (mounted) setState(() => _isUploadingImage = false);
    }
  }

  void _applyTemplate(BannerTemplate? tmpl) {
    if (tmpl == null) return;
    setState(() {
      _selectedTemplateId = tmpl.id;
      if (_titleCtrl.text.trim().isEmpty) {
        _titleCtrl.text = tmpl.titleTemplate;
      }
      if (_bodyCtrl.text.trim().isEmpty) {
        _bodyCtrl.text = tmpl.bodyTemplate;
      }
      if (tmpl.imageUrl != null &&
          tmpl.imageUrl!.isNotEmpty &&
          (_imageUrl == null || _imageUrl!.isEmpty)) {
        _imageUrl = tmpl.imageUrl;
      }
      if (_priorityCtrl.text.trim().isEmpty ||
          _priorityCtrl.text.trim() == '10') {
        _priorityCtrl.text = tmpl.defaultPriority.toString();
      }
    });
  }

  Future<void> _submit() async {
    final title = _titleCtrl.text.trim();
    final body = _bodyCtrl.text.trim();
    final deepLink = _deepLinkCtrl.text.trim();
    final priority = int.tryParse(_priorityCtrl.text.trim()) ?? 1;
    final displayOrder = int.tryParse(_displayOrderCtrl.text.trim()) ?? 0;

    if (title.isEmpty || body.isEmpty) {
      setState(() => _error = 'Judul dan isi pengumuman wajib diisi');
      return;
    }
    if (_scheduledStart != null &&
        _scheduledEnd != null &&
        !_scheduledEnd!.isAfter(_scheduledStart!)) {
      setState(() => _error = 'Jadwal berakhir harus setelah jadwal mulai');
      return;
    }
    if (deepLink.isNotEmpty) {
      final ok = _allowedDeepLinkPrefixes.any(
        (p) => deepLink == p || deepLink.startsWith('$p/'),
      );
      if (!ok) {
        setState(
          () => _error =
              'Tautan harus internal (contoh: /keuangan, /kegiatan, /pengumuman)',
        );
        return;
      }
    }

    setState(() {
      _isSaving = true;
      _error = null;
    });

    final payload = <String, dynamic>{
      'title': title,
      'body': body,
      'image_url': _imageUrl,
      'deep_link': deepLink.isEmpty ? null : deepLink,
      'template_id': _selectedTemplateId,
      'scheduled_start': _scheduledStart?.toUtc().toIso8601String(),
      'scheduled_end': _scheduledEnd?.toUtc().toIso8601String(),
      'priority': priority,
      'display_order': displayOrder,
    };

    try {
      final repo = ref.read(communityRepositoryProvider);
      if (_isEditing) {
        await repo.updateAnnouncement(widget.announcementId!, payload);
        if (!mounted) return;
        context.pop(true);
      } else {
        payload['publish_now'] = _publishNow;
        await repo.createAnnouncement(payload);
        if (!mounted) return;
        context.pop(true);
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _isSaving = false;
      });
    }
  }

  String _formatDateTime(DateTime? value) {
    if (value == null) return 'Tidak dijadwalkan';
    final d = value.toLocal();
    const months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];
    final day = d.day.toString().padLeft(2, '0');
    final hour = d.hour.toString().padLeft(2, '0');
    final minute = d.minute.toString().padLeft(2, '0');
    return '$day ${months[d.month - 1]} ${d.year}, $hour:$minute';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final textStyle = theme.textTheme.bodyMedium;

    InputDecoration decoration(String label, {String? hint}) => InputDecoration(
      labelText: label,
      hintText: hint,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppRadius.sm),
      ),
    );

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: MenuAppBarTitle(
          title: _isEditing ? 'Ubah Pengumuman' : 'Tambah Pengumuman',
        ),
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator.adaptive())
          : ListView(
              padding: const EdgeInsets.all(AppSpacing.base),
              children: [
                if (_error != null) ...[
                  Container(
                    padding: const EdgeInsets.all(12),
                    margin: const EdgeInsets.only(bottom: 12),
                    decoration: BoxDecoration(
                      color: Colors.red.withValues(alpha: 0.12),
                      border: Border.all(
                        color: Colors.red.withValues(alpha: 0.4),
                      ),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: Text(
                      _error!,
                      style: textStyle?.copyWith(color: AppColors.danger),
                    ),
                  ),
                ],

                // Template picker (create mode only)
                if (!_isEditing && _templates.isNotEmpty) ...[
                  Text(
                    'Template Banner (opsional)',
                    style: theme.textTheme.titleSmall,
                  ),
                  const SizedBox(height: 6),
                  DropdownButtonFormField<String?>(
                    value: _selectedTemplateId,
                    decoration: decoration('Pilih template'),
                    items: [
                      const DropdownMenuItem<String?>(
                        value: null,
                        child: Text('-- Kustom (tanpa template) --'),
                      ),
                      ..._templates.map(
                        (t) => DropdownMenuItem<String?>(
                          value: t.id,
                          child: Text('${t.name} (${t.category})'),
                        ),
                      ),
                    ],
                    onChanged: (value) {
                      BannerTemplate? tmpl;
                      if (value != null) {
                        for (final t in _templates) {
                          if (t.id == value) {
                            tmpl = t;
                            break;
                          }
                        }
                      }
                      _applyTemplate(tmpl);
                    },
                  ),
                  const SizedBox(height: AppSpacing.base),
                ],

                TextField(
                  controller: _titleCtrl,
                  decoration: decoration('Judul Pengumuman *'),
                  textInputAction: TextInputAction.next,
                ),
                const SizedBox(height: AppSpacing.base),
                TextField(
                  controller: _bodyCtrl,
                  decoration: decoration('Isi Pengumuman *'),
                  minLines: 4,
                  maxLines: 8,
                ),
                const SizedBox(height: AppSpacing.base),
                TextField(
                  controller: _deepLinkCtrl,
                  decoration: decoration(
                    'Tautan Internal (opsional)',
                    hint: 'Contoh: /keuangan, /kegiatan, /pengumuman',
                  ),
                ),
                const SizedBox(height: AppSpacing.base),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _priorityCtrl,
                        decoration: decoration('Prioritas'),
                        keyboardType: TextInputType.number,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextField(
                        controller: _displayOrderCtrl,
                        decoration: decoration('Urutan'),
                        keyboardType: TextInputType.number,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.base),
                Text(
                  'Masa Tayang (opsional)',
                  style: theme.textTheme.titleSmall,
                ),
                const SizedBox(height: 6),
                _scheduleTile(
                  label: 'Mulai tayang',
                  value: _scheduledStart,
                  onPick: () async {
                    final picked = await _pickDateTime(_scheduledStart);
                    if (picked != null) {
                      setState(() => _scheduledStart = picked);
                    }
                  },
                  onClear: () => setState(() => _scheduledStart = null),
                ),
                const SizedBox(height: 8),
                _scheduleTile(
                  label: 'Berakhir tayang',
                  value: _scheduledEnd,
                  onPick: () async {
                    final picked = await _pickDateTime(_scheduledEnd);
                    if (picked != null) {
                      setState(() => _scheduledEnd = picked);
                    }
                  },
                  onClear: () => setState(() => _scheduledEnd = null),
                ),
                const SizedBox(height: AppSpacing.base),

                Text(
                  'Gambar Banner (opsional)',
                  style: theme.textTheme.titleSmall,
                ),
                const SizedBox(height: 6),
                if (_imageUrl != null && _imageUrl!.isNotEmpty) ...[
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                    child: CommunityImage(
                      imageRef: _imageUrl!,
                      height: 140,
                      width: double.infinity,
                      fit: BoxFit.cover,
                    ),
                  ),
                  const SizedBox(height: 8),
                ],
                OutlinedButton.icon(
                  onPressed: _isUploadingImage ? null : _pickImage,
                  icon: _isUploadingImage
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.image_outlined),
                  label: Text(
                    _isUploadingImage ? 'Mengunggah...' : 'Pilih Gambar',
                  ),
                ),
                const SizedBox(height: AppSpacing.base),

                if (!_isEditing)
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('Publikasikan langsung'),
                    subtitle: const Text('Tampil di banner Beranda'),
                    value: _publishNow,
                    onChanged: (value) => setState(() => _publishNow = value),
                  ),

                const SizedBox(height: AppSpacing.base),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: _isSaving ? null : () => context.pop(false),
                      child: const Text('Batal'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: (_isSaving || _isUploadingImage)
                          ? null
                          : _submit,
                      child: Text(_isSaving ? 'Menyimpan...' : 'Simpan'),
                    ),
                  ],
                ),
              ],
            ),
    );
  }

  Widget _scheduleTile({
    required String label,
    required DateTime? value,
    required VoidCallback onPick,
    required VoidCallback onClear,
  }) {
    return InkWell(
      onTap: onPick,
      borderRadius: BorderRadius.circular(AppRadius.sm),
      child: AppCard(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          child: Row(
            children: [
              const Icon(Icons.schedule, size: 18),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      label,
                      style: const TextStyle(fontSize: 11, color: Colors.grey),
                    ),
                    Text(_formatDateTime(value)),
                  ],
                ),
              ),
              if (value != null)
                IconButton(
                  tooltip: 'Hapus jadwal',
                  icon: const Icon(Icons.close, size: 18),
                  onPressed: onClear,
                ),
            ],
          ),
        ),
      ),
    );
  }
}
