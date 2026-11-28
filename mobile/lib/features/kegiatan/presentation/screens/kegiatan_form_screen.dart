import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/navigators/form_navigation_guard.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/banner_style_editor.dart';

class KegiatanFormScreen extends ConsumerStatefulWidget {
  final String? activityId;
  final CommunityActivity? initialActivity;

  const KegiatanFormScreen({
    super.key,
    this.activityId,
    this.initialActivity,
  });

  @override
  ConsumerState<KegiatanFormScreen> createState() => _KegiatanFormScreenState();
}

class _KegiatanFormScreenState extends ConsumerState<KegiatanFormScreen> {
  final _titleCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final _locationCtrl = TextEditingController();
  final _picNameCtrl = TextEditingController();
  final _budgetCtrl = TextEditingController();

  DateTime _startDate = DateTime.now().add(const Duration(days: 3));
  DateTime? _endDate;
  bool _showOnBanner = false;
  bool _publishNow = false;
  String? _posterUrl;
  BannerStyleConfig? _styleConfig;

  bool _isLoading = false;
  bool _isSaving = false;
  String? _error;

  late final FormSnapshot _snapshot;
  FormGuardRegistration? _guardReg;

  bool get _isEditing =>
      widget.activityId != null && widget.activityId!.isNotEmpty;

  String get _routePath => _isEditing
      ? '/home/kegiatan/${widget.activityId}/edit'
      : '/home/kegiatan/create';

  Map<String, Object?> _currentValues() => {
    'title': _titleCtrl.text,
    'desc': _descCtrl.text,
    'location': _locationCtrl.text,
    'picName': _picNameCtrl.text,
    'budget': _budgetCtrl.text,
    'startDate': _startDate,
    'endDate': _endDate,
    'showOnBanner': _showOnBanner,
    'publishNow': _publishNow,
    'posterUrl': _posterUrl,
    'styleConfig': _styleConfig?.toJson(),
  };

  bool get _isDirty => !_isLoading && _snapshot.isDirty(_currentValues());

  @override
  void initState() {
    super.initState();
    _snapshot = FormSnapshot(_currentValues());
    _guardReg = FormNavigationGuard.register(
      _routePath,
      isDirty: () => _isDirty,
      isSaving: () => _isSaving,
    );
    if (widget.initialActivity != null) {
      final existing = widget.initialActivity!;
      _titleCtrl.text = existing.title;
      _descCtrl.text = existing.description;
      _locationCtrl.text = existing.location;
      _picNameCtrl.text = existing.picName ?? '';
      _budgetCtrl.text = existing.budgetPlanned ?? '';
      _startDate = existing.startDate.toLocal();
      _endDate = existing.endDate?.toLocal();
      _showOnBanner = existing.showOnBanner;
      _publishNow = existing.publicationStatus == 'published';
      _posterUrl = existing.posterUrl;
      _styleConfig = existing.styleConfig ??
          ((existing.posterUrl != null && existing.posterUrl!.isNotEmpty)
              ? BannerStyleConfig(
                  backgroundType: 'image',
                  image: ImageConfig(url: existing.posterUrl!, zoom: 1.0),
                  showDetail: true,
                )
              : BannerStyleConfig(
                  backgroundType: 'color',
                  backgroundColor: '#A970FF',
                  gradient: computeNeonGradientConfig('#A970FF'),
                  showDetail: true,
                ));
      _initSnapshot();
      return;
    }
    if (Platform.environment.containsKey('FLUTTER_TEST')) {
      _initSnapshot();
      return;
    }
    _bootstrap();
  }

  void _initSnapshot() {
    _snapshot.reset(_currentValues());
  }

  Future<void> _bootstrap() async {
    setState(() => _isLoading = true);
    final repo = ref.read(communityRepositoryProvider);
    try {
      CommunityActivity? existing;
      if (_isEditing) {
        existing = await repo.getActivity(widget.activityId!);
      }
      if (!mounted) return;

      setState(() {
        if (existing != null) {
          _titleCtrl.text = existing.title;
          _descCtrl.text = existing.description;
          _locationCtrl.text = existing.location;
          _picNameCtrl.text = existing.picName ?? '';
          _budgetCtrl.text = existing.budgetPlanned ?? '';
          _startDate = existing.startDate.toLocal();
          _endDate = existing.endDate?.toLocal();
          _showOnBanner = existing.showOnBanner;
          _posterUrl = existing.posterUrl;
          _styleConfig = existing.styleConfig ??
              ((existing.posterUrl != null && existing.posterUrl!.isNotEmpty)
                  ? BannerStyleConfig(
                      backgroundType: 'image',
                      image: ImageConfig(url: existing.posterUrl!, zoom: 1.0),
                      showDetail: true,
                    )
                  : BannerStyleConfig(
                      backgroundType: 'color',
                      backgroundColor: '#A970FF',
                      gradient: computeNeonGradientConfig('#A970FF'),
                      showDetail: true,
                    ));
          _publishNow = existing.publicationStatus == 'published';
        } else {
          _styleConfig = BannerStyleConfig(
            backgroundType: 'color',
            backgroundColor: '#A970FF',
            gradient: computeNeonGradientConfig('#A970FF'),
            showDetail: true,
          );
        }
        _initSnapshot();
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
    FormNavigationGuard.unregister(_routePath, registration: _guardReg);
    _titleCtrl.dispose();
    _descCtrl.dispose();
    _locationCtrl.dispose();
    _picNameCtrl.dispose();
    _budgetCtrl.dispose();
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

  Future<void> _handlePop() async {
    await FormNavigationGuard.requestFormCancel(
      context,
      routePath: _routePath,
      isDirtyOverride: () => _isDirty,
      isSavingOverride: () => _isSaving,
      fallbackRoute: '/home/kegiatan',
      result: false,
    );
  }

  Future<void> _submit() async {
    final title = _titleCtrl.text.trim();
    final location = _locationCtrl.text.trim();
    final description = _descCtrl.text.trim();
    final picName = _picNameCtrl.text.trim();
    final budget = _budgetCtrl.text.trim();

    if (title.isEmpty || location.isEmpty) {
      setState(() => _error = 'Judul dan lokasi kegiatan wajib diisi');
      return;
    }
    if (_endDate != null && !_endDate!.isAfter(_startDate)) {
      setState(() => _error = 'Waktu selesai harus setelah waktu mulai');
      return;
    }

    setState(() {
      _isSaving = true;
      _error = null;
    });

    final effectivePosterUrl = _showOnBanner && _styleConfig?.backgroundType == 'image'
        ? (_styleConfig?.image?.url ?? _posterUrl)
        : _posterUrl;

    final payload = <String, dynamic>{
      'title': title,
      'location': location,
      'description': description.isEmpty ? null : description,
      'pic_name': picName.isEmpty ? null : picName,
      'budget_planned': budget.isEmpty ? null : budget,
      'starts_at': _startDate.toUtc().toIso8601String(),
      'ends_at': _endDate?.toUtc().toIso8601String(),
      'show_on_banner': _showOnBanner,
      'poster_url': effectivePosterUrl,
      'style_config': _showOnBanner && _styleConfig != null
          ? _styleConfig!.toJson()
          : null,
    };

    try {
      final repo = ref.read(communityRepositoryProvider);
      if (_isEditing) {
        await repo.updateActivity(widget.activityId!, payload);
        if (!mounted) return;
        FormNavigationGuard.exitAfterSave(
          context,
          routePath: _routePath,
          fallbackRoute: '/home/kegiatan',
          result: true,
        );
      } else {
        payload['publication_status'] = _publishNow ? 'published' : 'draft';
        await repo.createActivity(payload);
        if (!mounted) return;
        FormNavigationGuard.exitAfterSave(
          context,
          routePath: _routePath,
          fallbackRoute: '/home/kegiatan',
          result: true,
        );
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

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        await _handlePop();
      },
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: _handlePop,
          ),
          title: Text(
            _isEditing ? 'Ubah Kegiatan' : 'Tambah Kegiatan',
          ),
        ),
        body: _isLoading
            ? const Center(child: CircularProgressIndicator.adaptive())
            : SingleChildScrollView(
                padding: const EdgeInsets.all(AppSpacing.base),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
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

                  TextField(
                    controller: _titleCtrl,
                    decoration: decoration('Judul Kegiatan *'),
                    textInputAction: TextInputAction.next,
                  ),
                  const SizedBox(height: AppSpacing.base),

                  TextField(
                    controller: _locationCtrl,
                    decoration: decoration('Lokasi Kegiatan *'),
                    textInputAction: TextInputAction.next,
                  ),
                  const SizedBox(height: AppSpacing.base),

                  TextField(
                    controller: _descCtrl,
                    decoration: decoration('Deskripsi Kegiatan'),
                    minLines: 3,
                    maxLines: 6,
                  ),
                  const SizedBox(height: AppSpacing.base),

                  TextField(
                    controller: _picNameCtrl,
                    decoration: decoration('Nama Penanggung Jawab (PIC)'),
                    textInputAction: TextInputAction.next,
                  ),
                  const SizedBox(height: AppSpacing.base),

                  TextField(
                    controller: _budgetCtrl,
                    decoration: decoration('Rencana Anggaran (Rp)'),
                    keyboardType: TextInputType.number,
                  ),
                  const SizedBox(height: AppSpacing.base),

                  Text(
                    'Waktu Pelaksanaan',
                    style: theme.textTheme.titleSmall,
                  ),
                  const SizedBox(height: 6),
                  _scheduleTile(
                    label: 'Waktu Mulai *',
                    value: _startDate,
                    onPick: () async {
                      final picked = await _pickDateTime(_startDate);
                      if (picked != null) {
                        setState(() => _startDate = picked);
                      }
                    },
                    onClear: null,
                  ),
                  const SizedBox(height: 8),
                  _scheduleTile(
                    label: 'Waktu Selesai (Opsional)',
                    value: _endDate,
                    onPick: () async {
                      final picked = await _pickDateTime(_endDate ?? _startDate.add(const Duration(hours: 2)));
                      if (picked != null) {
                        setState(() => _endDate = picked);
                      }
                    },
                    onClear: () => setState(() => _endDate = null),
                  ),
                  const SizedBox(height: AppSpacing.base),

                  if (_posterUrl != null && _posterUrl!.isNotEmpty) ...[
                    Text(
                      'Poster Kegiatan Terlampir',
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 6),
                    ClipRRect(
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                      child: Container(
                        width: double.infinity,
                        height: 120,
                        color: Colors.grey.withValues(alpha: 0.1),
                        child: CommunityImage(
                          imageRef: _posterUrl!,
                          fit: BoxFit.contain,
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.base),
                  ],

                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('Tampilkan di Banner Beranda'),
                    subtitle: const Text('Muncul sebagai banner kegiatan di halaman utama'),
                    value: _showOnBanner,
                    onChanged: (val) => setState(() => _showOnBanner = val),
                  ),

                  if (_showOnBanner) ...[
                    const SizedBox(height: 8),
                    Text(
                      'Desain & Background Banner',
                      style: theme.textTheme.titleSmall,
                    ),
                    const SizedBox(height: 8),
                    BannerStyleEditor(
                      value: _styleConfig,
                      titlePreview: _titleCtrl.text.isEmpty
                          ? 'Judul Kegiatan'
                          : _titleCtrl.text,
                      bodyPreview: _locationCtrl.text.isEmpty
                          ? 'Lokasi Kegiatan'
                          : '📍 ${_locationCtrl.text}',
                      category: 'activity',
                      onChanged: (newConfig) {
                        setState(() {
                          _styleConfig = newConfig;
                          if (newConfig.backgroundType == 'image' &&
                              newConfig.image?.url != null) {
                            _posterUrl = newConfig.image!.url;
                          }
                        });
                      },
                    ),
                  ],
                  const SizedBox(height: AppSpacing.base),

                  if (!_isEditing)
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('Publikasikan langsung'),
                      subtitle: const Text('Langsung berstatus published'),
                      value: _publishNow,
                      onChanged: (value) => setState(() => _publishNow = value),
                    ),

                  const SizedBox(height: AppSpacing.base),
                  Row(
                    children: [
                      Expanded(
                        child: SizedBox(
                          height: 48,
                          child: ElevatedButton(
                            onPressed: _isSaving ? null : _handlePop,
                            style: ElevatedButton.styleFrom(
                              backgroundColor: AppColors.danger,
                              foregroundColor: Colors.white,
                              elevation: 0,
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppRadius.base),
                              ),
                            ),
                            child: const Text('Batal'),
                          ),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: SizedBox(
                          height: 48,
                          child: ElevatedButton(
                            onPressed: _isSaving ? null : _submit,
                            style: ElevatedButton.styleFrom(
                              backgroundColor: AppColors.success,
                              foregroundColor: Colors.white,
                              elevation: 0,
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppRadius.base),
                              ),
                            ),
                            child: Text(_isSaving ? 'Menyimpan...' : 'Simpan'),
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
      ),
    );
  }

  Widget _scheduleTile({
    required String label,
    required DateTime? value,
    required VoidCallback onPick,
    required VoidCallback? onClear,
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
              if (value != null && onClear != null)
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
