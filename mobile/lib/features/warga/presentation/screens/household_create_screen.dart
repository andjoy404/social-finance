import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import '../../data/api_warga_models.dart';
import '../../data/warga_repository.dart';
import '../../data/warga_providers.dart';
import '../../../iuran/data/iuran_providers.dart';

/// Screen for creating a new household (adding a warga/head of family).
class HouseholdCreateScreen extends ConsumerStatefulWidget {
  final bool isSuperAdmin;

  const HouseholdCreateScreen({super.key, required this.isSuperAdmin});

  @override
  ConsumerState<HouseholdCreateScreen> createState() =>
      _HouseholdCreateScreenState();
}

class _HouseholdCreateScreenState extends ConsumerState<HouseholdCreateScreen> {
  final _formKey = GlobalKey<FormState>();
  final _houseNumberCtrl = TextEditingController();
  final _headNameCtrl = TextEditingController();
  final _nikCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();
  final _passwordCtrl = TextEditingController();
  final _addressCtrl = TextEditingController();

  String _occupancyStatus = 'OWNER';
  String _startDate = '';
  String? _endDate;
  bool _obscurePassword = true;
  bool _isActive = true;
  String? _selectedRtId;
  bool _saving = false;
  List<Map<String, dynamic>> _rtOptions = [];
  bool _rtLoading = false;
  String? _error;
  List<String> _validationErrors = [];

  @override
  void initState() {
    super.initState();
    _nikCtrl.addListener(() {
      if (_nikCtrl.text.length > 16) {
        _nikCtrl.text = _nikCtrl.text.substring(0, 16);
        _nikCtrl.selection = TextSelection.collapsed(
          offset: _nikCtrl.text.length,
        );
      }
    });
  }

  @override
  void dispose() {
    _houseNumberCtrl.dispose();
    _headNameCtrl.dispose();
    _nikCtrl.dispose();
    _phoneCtrl.dispose();
    _emailCtrl.dispose();
    _passwordCtrl.dispose();
    _addressCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadRts() async {
    if (!widget.isSuperAdmin) return;

    setState(() => _rtLoading = true);
    try {
      final repo = ref.read(wargaRepositoryProvider);
      final rts = await repo.fetchRts();
      final filteredRts = rts
          .where((r) => ['002', '003'].contains(r['rt'] as String?))
          .toList();
      if (mounted) {
        setState(() => _rtOptions = filteredRts);
      }
    } catch (e) {
      // Silently fail — RT selection not critical for non-superadmin
    } finally {
      if (mounted) setState(() => _rtLoading = false);
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _loadRts();
  }

  bool _validate() {
    final errors = <String>[];

    if (_houseNumberCtrl.text.trim().isEmpty) {
      errors.add('Nomor rumah harus diisi.');
    }
    if (_headNameCtrl.text.trim().isEmpty) {
      errors.add('Nama lengkap warga harus diisi.');
    }
    if (_nikCtrl.text.trim().isNotEmpty &&
        !RegExp(r'^[0-9]{16}$').hasMatch(_nikCtrl.text.trim())) {
      errors.add('NIK harus berupa 16 digit angka.');
    }
    if (_emailCtrl.text.trim().isNotEmpty &&
        !RegExp(r'^[^\s@]+@[^\s@]+\.[^\s@]+$').hasMatch(_emailCtrl.text.trim())) {
      errors.add('Format email tidak valid.');
    }
    if (_startDate.isEmpty) {
      _startDate = DateTime.now().toLocal().toString().split(' ')[0];
    }
    if (_endDate != null && _endDate!.isNotEmpty && _startDate.isNotEmpty) {
      final start = DateTime.tryParse(_startDate);
      final end = DateTime.tryParse(_endDate!);
      if (start != null && end != null && !end.isAfter(start)) {
        errors.add('Tanggal selesai tidak boleh sebelum tanggal mulai.');
      }
    }
    if (_passwordCtrl.text.isNotEmpty && _passwordCtrl.text.length < 8) {
      errors.add('Password minimal 8 karakter.');
    }
    if (widget.isSuperAdmin && _selectedRtId == null) {
      errors.add('RT Tujuan harus dipilih.');
    }

    setState(() => _validationErrors = errors);
    return errors.isEmpty;
  }

  Future<void> _save() async {
    if (!_validate()) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    try {
      final repo = ref.read(wargaRepositoryProvider);
      final request = CreateHouseholdRequest(
        houseNumber: _houseNumberCtrl.text.trim(),
        headName: _headNameCtrl.text.trim(),
        nik: _nikCtrl.text.trim(),
        phone: _phoneCtrl.text.trim(),
        email: _emailCtrl.text.trim(),
        occupancyStatus: _occupancyStatus,
        startDate: _startDate,
        endDate: _endDate,
        password: _passwordCtrl.text.trim().isEmpty
            ? null
            : _passwordCtrl.text.trim(),
        address: _addressCtrl.text.trim().isEmpty
            ? null
            : _addressCtrl.text.trim(),
        isActive: _isActive,
      );

      await repo.createHousehold(
        request: request,
        rtId: widget.isSuperAdmin ? _selectedRtId : null,
      );

      // Refresh warga list so the new household appears immediately
      await ref.read(wargaListProvider.notifier).refresh();

      // Refresh overdue warning summary
      ref.read(overdueWarningSummaryProvider.notifier).refresh();

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Data warga berhasil ditambahkan.'),
            backgroundColor: AppColors.success,
          ),
        );
        // Successful save — allow the subsequent pop through the route guard.
        final routePath = GoRouterState.of(context).uri.path;
        KasFormNavigationGuard.allowExitFor(routePath);
        try {
          context.pop();
        } catch (_) {
          KasFormNavigationGuard.revokeExitFor(routePath);
          rethrow;
        }
      }
    } catch (e) {
      final error = ref.read(wargaRepositoryProvider).mapDioError(e);
      if (mounted) {
        setState(() => _error = error.message);
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  /// Show confirmation dialog before canceling the form.
  /// Used by AppBar back, Android system back, and the Batal button.

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        await KasFormNavigationGuard.requestFormCancel(context);
      },
      child: Scaffold(
        appBar: AppBar(title: const Text('Tambah Warga')),
        bottomNavigationBar: SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            child: LayoutBuilder(
              builder: (context, constraints) {
                final availableWidth = constraints.maxWidth;
                final gap = AppSpacing.md;
                final buttonWidth = (availableWidth - 32 - gap) / 2;
                return Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    SizedBox(
                      width: buttonWidth > 0
                          ? buttonWidth
                          : availableWidth - 32,
                      child: _buildSimpanButton(),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    SizedBox(
                      width: buttonWidth > 0
                          ? buttonWidth
                          : availableWidth - 32,
                      child: _buildBatalButton(),
                    ),
                  ],
                );
              },
            ),
          ),
        ),
        body: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppSpacing.base),
            children: [
              // Error message
              if (_error != null)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(
                      alpha: isDark ? 0.2 : 0.1,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    border: Border.all(
                      color: AppColors.danger.withValues(
                        alpha: isDark ? 0.4 : 0.2,
                      ),
                    ),
                  ),
                  child: Text(
                    _error!,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: AppColors.danger,
                    ),
                  ),
                ),
              if (_error != null) const SizedBox(height: AppSpacing.md),

              // Validation errors
              if (_validationErrors.isNotEmpty)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(
                      alpha: isDark ? 0.15 : 0.08,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Data belum lengkap:',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                          color: AppColors.danger,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      ..._validationErrors.map(
                        (e) => Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('• ', style: TextStyle(fontSize: 16)),
                              Expanded(child: Text(e)),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),

              // RT selector for superadmin
              if (widget.isSuperAdmin) ...[
                const SizedBox(height: AppSpacing.lg),
                Text(
                  'RT Tujuan',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                if (_rtLoading)
                  const Center(child: CircularProgressIndicator())
                else if (_rtOptions.isEmpty)
                  Text(
                    'Tidak ada RT yang tersedia.',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  )
                else
                  DropdownButtonFormField<String>(
                    decoration: InputDecoration(
                      labelText: 'Pilih RT',
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: AppSpacing.md,
                        vertical: AppSpacing.sm,
                      ),
                    ),
                    initialValue: _selectedRtId,
                    hint: const Text('Pilih RT'),
                    items: _rtOptions.map((rt) {
                      final rtNum = ((rt['rt'] as String?) ?? '').padLeft(
                        3,
                        '0',
                      );
                      final rwNum =
                          ((rt['rw'] as num?)?.toInt().toString().padLeft(
                            3,
                            '0',
                          )) ??
                          '';
                      final name = (rt['name'] as String?) ?? 'RT tanpa nama';
                      return DropdownMenuItem(
                        value: rt['id'] as String?,
                        child: Text('RT $rtNum / RW $rwNum — $name'),
                      );
                    }).toList(),
                    onChanged: widget.isSuperAdmin
                        ? (value) => setState(() => _selectedRtId = value)
                        : null,
                    validator: (value) {
                      if (widget.isSuperAdmin &&
                          (value == null || value.isEmpty)) {
                        return 'RT Tujuan harus dipilih.';
                      }
                      return null;
                    },
                  ),
                const SizedBox(height: AppSpacing.lg),
              ],

              // Nomor Rumah + Status Hunian
              Row(
                children: [
                  Expanded(
                    flex: 2,
                    child: AppTextField(
                      controller: _houseNumberCtrl,
                      label: 'Nomor Rumah',
                      hintText: 'Contoh: 001',
                      keyboardType: TextInputType.text,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    flex: 3,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Status Hunian',
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        SegmentedButton<String>(
                          segments: [
                            ButtonSegment(
                              value: 'OWNER',
                              label: const Text(
                                'Pemilik',
                                style: TextStyle(fontSize: 12),
                              ),
                              icon: const Icon(Icons.home, size: 16),
                            ),
                            ButtonSegment(
                              value: 'TENANT',
                              label: const Text(
                                'Penyewa',
                                style: TextStyle(fontSize: 12),
                              ),
                              icon: const Icon(Icons.hail, size: 16),
                            ),
                          ],
                          selected: {_occupancyStatus},
                          onSelectionChanged: (selected) {
                            setState(() => _occupancyStatus = selected.first);
                          },
                          style: SegmentedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 4,
                            ),
                            backgroundColor: isDark
                                ? AppColors.darkSurface
                                : AppColors.lightSurface,
                            foregroundColor: isDark
                                ? AppColors.darkText
                                : AppColors.lightText,
                            selectedForegroundColor: AppColors.accent,
                            selectedBackgroundColor: AppColors.accentSoftColor(
                              context,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Nama Lengkap Warga
              AppTextField(
                controller: _headNameCtrl,
                label: 'Nama Lengkap Warga',
                hintText: 'Nama lengkap',
              ),
              const SizedBox(height: AppSpacing.lg),

              // NIK
              AppTextField(
                controller: _nikCtrl,
                label: 'NIK',
                hintText: '16 digit',
                keyboardType: TextInputType.number,
                inputFormatters: [
                  FilteringTextInputFormatter.digitsOnly,
                  LengthLimitingTextInputFormatter(16),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Nomor Telepon
              AppTextField(
                controller: _phoneCtrl,
                label: 'Nomor Telepon',
                hintText: '08123456789',
                keyboardType: TextInputType.phone,
              ),
              const SizedBox(height: AppSpacing.lg),

              // Email
              AppTextField(
                controller: _emailCtrl,
                label: 'Email',
                hintText: 'nama@email.com',
                keyboardType: TextInputType.emailAddress,
              ),
              const SizedBox(height: AppSpacing.lg),

              // Tanggal Mulai Menempati Rumah
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Tanggal Mulai Menempati Rumah',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  InkWell(
                    onTap: () async {
                      final picked = await showDatePicker(
                        context: context,
                        initialDate: DateTime.now(),
                        firstDate: DateTime(2000),
                        lastDate: DateTime.now().add(const Duration(days: 365)),
                      );
                      if (picked != null) {
                        setState(() {
                          _startDate = picked.toLocal().toString().split(
                            ' ',
                          )[0];
                        });
                      }
                    },
                    child: Container(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      decoration: BoxDecoration(
                        border: Border.all(color: AppColors.lightBorder),
                        borderRadius: BorderRadius.circular(AppRadius.base),
                        color: isDark
                            ? AppColors.darkSurface
                            : AppColors.lightSurface,
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.calendar_today,
                            size: 20,
                            color: AppColors.accent,
                          ),
                          const SizedBox(width: AppSpacing.md),
                          Text(
                            _startDate.isEmpty ? 'Pilih tanggal' : _startDate,
                            style: theme.textTheme.bodyLarge,
                          ),
                          const Spacer(),
                          const Icon(
                            Icons.arrow_forward_ios,
                            size: 16,
                            color: Colors.grey,
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Tanggal Selesai Hunian (opsional)
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Tanggal Selesai Hunian (opsional)',
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      if (_endDate != null && _endDate!.isNotEmpty)
                        TextButton(
                          onPressed: () => setState(() => _endDate = null),
                          child: const Text(
                            'Hapus',
                            style: TextStyle(
                              color: AppColors.danger,
                              fontSize: 12,
                            ),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  InkWell(
                    onTap: () async {
                      final now = DateTime.now();
                      final initial = _endDate != null && _endDate!.isNotEmpty
                          ? (DateTime.tryParse(_endDate!) ?? now)
                          : (_startDate.isNotEmpty
                                ? (DateTime.tryParse(_startDate) ?? now)
                                : now);
                      final picked = await showDatePicker(
                        context: context,
                        initialDate: initial,
                        firstDate: DateTime(2000),
                        lastDate: DateTime.now().add(
                          const Duration(days: 365 * 10),
                        ),
                      );
                      if (picked != null) {
                        setState(() {
                          _endDate = picked.toLocal().toString().split(' ')[0];
                        });
                      }
                    },
                    child: Container(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      decoration: BoxDecoration(
                        border: Border.all(color: AppColors.lightBorder),
                        borderRadius: BorderRadius.circular(AppRadius.base),
                        color: isDark
                            ? AppColors.darkSurface
                            : AppColors.lightSurface,
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.calendar_today,
                            size: 20,
                            color: AppColors.accent,
                          ),
                          const SizedBox(width: AppSpacing.md),
                          Text(
                            (_endDate == null || _endDate!.isEmpty)
                                ? 'Pilih tanggal selesai'
                                : _endDate!,
                            style: theme.textTheme.bodyLarge?.copyWith(
                              color: (_endDate == null || _endDate!.isEmpty)
                                  ? (isDark
                                        ? AppColors.darkTextMuted
                                        : AppColors.lightTextMuted)
                                  : null,
                            ),
                          ),
                          const Spacer(),
                          const Icon(
                            Icons.arrow_forward_ios,
                            size: 16,
                            color: Colors.grey,
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Buat Password
              AppTextField(
                controller: _passwordCtrl,
                label: 'Buat Password',
                hintText: 'Minimal 8 karakter (opsional)',
                obscureText: _obscurePassword,
                suffixIcon: IconButton(
                  icon: Icon(
                    _obscurePassword ? Icons.visibility_off : Icons.visibility,
                    color: AppColors.accent,
                    size: 20,
                  ),
                  onPressed: () =>
                      setState(() => _obscurePassword = !_obscurePassword),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),

              // Alamat (optional)
              AppTextField(
                controller: _addressCtrl,
                label: 'Alamat (opsional)',
                hintText: 'Jl. Mawar No. 1',
              ),
              const SizedBox(height: AppSpacing.lg),

              // Status
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Status',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  SegmentedButton<bool>(
                    segments: [
                      ButtonSegment(
                        value: true,
                        label: const Text('Aktif'),
                        icon: const Icon(Icons.check_circle, size: 18),
                      ),
                      ButtonSegment(
                        value: false,
                        label: const Text('Tidak Aktif'),
                        icon: const Icon(Icons.cancel, size: 18),
                      ),
                    ],
                    selected: {_isActive},
                    onSelectionChanged: (selected) {
                      setState(() => _isActive = selected.first);
                    },
                    style: SegmentedButton.styleFrom(
                      backgroundColor: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurface,
                      foregroundColor: isDark
                          ? AppColors.darkText
                          : AppColors.lightText,
                      selectedForegroundColor: AppColors.accent,
                      selectedBackgroundColor: AppColors.accentSoftColor(
                        context,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.xxl),

              const SizedBox.shrink(),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSimpanButton() => SizedBox(
    height: 48,
    child: ElevatedButton(
      onPressed: _saving ? null : _save,
      style: ElevatedButton.styleFrom(
        backgroundColor: AppColors.success,
        foregroundColor: Colors.white,
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 10),
        visualDensity: VisualDensity.compact,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
        ),
      ),
      child: _saving
          ? SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                backgroundColor: Colors.transparent,
                valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
              ),
            )
          : const Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(Icons.save_outlined, size: 18),
                SizedBox(width: 6),
                Text('Simpan'),
              ],
            ),
    ),
  );

  Widget _buildBatalButton() => SizedBox(
    height: 48,
    child: OutlinedButton(
      onPressed: _saving
          ? null
          : () => KasFormNavigationGuard.requestFormCancel(context),
      style: OutlinedButton.styleFrom(
        backgroundColor: AppColors.danger,
        foregroundColor: Colors.white,
        side: const BorderSide(color: AppColors.danger),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.base),
        ),
      ),
      child: const Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.close, size: 18),
          SizedBox(width: 6),
          Text('Batal'),
        ],
      ),
    ),
  );
}
