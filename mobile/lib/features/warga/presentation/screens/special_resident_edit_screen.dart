import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import '../../data/api_warga_models.dart';
import '../../data/warga_repository.dart';
import '../../data/warga_providers.dart';

/// Screen for editing an existing special resident (petugas khusus).
class SpecialResidentEditScreen extends ConsumerStatefulWidget {
  final String residentId;

  const SpecialResidentEditScreen({super.key, required this.residentId});

  @override
  ConsumerState<SpecialResidentEditScreen> createState() =>
      _SpecialResidentEditScreenState();
}

class _SpecialResidentEditScreenState
    extends ConsumerState<SpecialResidentEditScreen> {
  final _formKey = GlobalKey<FormState>();
  final _fullNameCtrl = TextEditingController();
  final _nikCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();

  String _jabatan = 'keamanan';
  bool _isActive = true;
  bool _loading = true;
  bool _saving = false;
  String? _error;
  List<String> _validationErrors = [];
  bool _notFound = false;

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
    _loadSpecialResident();
  }

  @override
  void dispose() {
    _fullNameCtrl.dispose();
    _nikCtrl.dispose();
    _phoneCtrl.dispose();
    _emailCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadSpecialResident() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final repository = ref.read(wargaRepositoryProvider);

      final resident = await repository.fetchResidentById(widget.residentId);

      if (!mounted) return;

      // Verify this is a special resident (has jabatan)
      if (resident.jabatan == null || resident.jabatan!.isEmpty) {
        throw const FormatException('Not a special resident');
      }

      // Map to MappedResident for field access
      final mapped = MappedResident.fromBackend(resident, null);

      setState(() {
        _fullNameCtrl.text = mapped.name;
        _nikCtrl.text = mapped.nik ?? '';
        _phoneCtrl.text = mapped.phoneNumber ?? '';
        _emailCtrl.text = mapped.email ?? '';
        _jabatan = mapped.jabatan ?? 'keamanan';
        _isActive = mapped.isActive;
        _loading = false;
      });
    } on FormatException catch (e) {
      if (e.message.contains('Not a special resident')) {
        if (mounted) {
          setState(() {
            _notFound = true;
            _loading = false;
          });
        }
      } else {
        if (mounted) {
          setState(() {
            _error = 'Data petugas tidak ditemukan.';
            _loading = false;
          });
        }
      }
    } catch (e) {
      final error = ref.read(wargaRepositoryProvider).mapDioError(e);
      if (mounted) {
        setState(() {
          _error = error.message;
          _loading = false;
        });
      }
    }
  }

  bool _validate() {
    final errors = <String>[];

    if (_fullNameCtrl.text.trim().isEmpty) {
      errors.add('Nama harus diisi.');
    }
    if (_nikCtrl.text.trim().isEmpty) {
      errors.add('NIK harus diisi.');
    } else if (!RegExp(r'^[0-9]{16}$').hasMatch(_nikCtrl.text.trim())) {
      errors.add('NIK harus berupa 16 digit angka.');
    }
    if (_phoneCtrl.text.trim().isEmpty) {
      errors.add('Telepon harus diisi.');
    }
    if (_emailCtrl.text.trim().isEmpty) {
      errors.add('Email harus diisi.');
    } else if (!RegExp(
      r'^[^\s@]+@[^\s@]+\.[^\s@]+$',
    ).hasMatch(_emailCtrl.text.trim())) {
      errors.add('Format email tidak valid.');
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
      final request = UpdateSpecialResidentRequest(
        fullName: _fullNameCtrl.text.trim().isEmpty
            ? null
            : _fullNameCtrl.text.trim(),
        nik: _nikCtrl.text.trim().isEmpty ? null : _nikCtrl.text.trim(),
        phone: _phoneCtrl.text.trim().isEmpty ? null : _phoneCtrl.text.trim(),
        email: _emailCtrl.text.trim().isEmpty ? null : _emailCtrl.text.trim(),
        jabatan: _jabatan.isEmpty ? null : _jabatan,
        isActive: _isActive,
      );

      final repo = ref.read(wargaRepositoryProvider);
      await repo.updateSpecialResident(widget.residentId, request: request);

      // Refresh warga list
      await ref.read(wargaListProvider.notifier).refresh();

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Data petugas berhasil diperbarui.'),
            backgroundColor: AppColors.success,
          ),
        );
        context.pop();
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
  Future<void> _confirmCancel() async {
    final shouldCancel = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Apakah Anda yakin ingin membatalkan?'),
        content: const Text('Perubahan yang belum disimpan akan hilang.'),
        actions: [
          Align(
            alignment: Alignment.center,
            child: SizedBox(
              height: 48,
              child: OutlinedButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: AppColors.accent),
                  backgroundColor: AppColors.accentSoftColor(context),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                ),
                child: const Text('Tidak, Tetap di Halaman'),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.center,
            child: Container(
              decoration: BoxDecoration(
                boxShadow: [
                  BoxShadow(
                    color: AppColors.danger.withValues(alpha: 0.15),
                    blurRadius: 8,
                    offset: const Offset(0, 0),
                  ),
                ],
              ),
              child: SizedBox(
                height: 48,
                child: OutlinedButton(
                  onPressed: () => Navigator.of(dialogContext).pop(true),
                  style: OutlinedButton.styleFrom(
                    side: BorderSide(color: AppColors.danger),
                    backgroundColor:
                        Theme.of(dialogContext).brightness == Brightness.dark
                        ? AppColors.darkDangerSoft
                        : AppColors.danger.withValues(alpha: 0.20),
                    foregroundColor: AppColors.danger,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                  ),
                  child: const Text('Ya, Batalkan'),
                ),
              ),
            ),
          ),
        ],
      ),
    );

    if (shouldCancel == true && mounted) {
      context.pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    if (_loading) {
      return Scaffold(
        appBar: AppBar(title: const Text('Edit Petugas')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_notFound) {
      return Scaffold(
        appBar: AppBar(title: const Text('Edit Petugas')),
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline,
                size: 64,
                color: theme.colorScheme.error.withValues(alpha: 0.5),
              ),
              const SizedBox(height: 16),
              Text(
                'Data petugas tidak ditemukan.',
                style: theme.textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Kembali'),
              ),
            ],
          ),
        ),
      );
    }

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        await _confirmCancel();
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Edit Petugas'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => _confirmCancel(),
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

              // Jenis Petugas
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Jenis Petugas',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  DropdownButtonFormField<String>(
                    initialValue: _jabatan,
                    decoration: InputDecoration(
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: AppSpacing.md,
                        vertical: AppSpacing.sm,
                      ),
                    ),
                    items: const [
                      DropdownMenuItem(
                        value: 'keamanan',
                        child: Text('Seksi Keamanan'),
                      ),
                      DropdownMenuItem(
                        value: 'kebersihan_pembangunan',
                        child: Text('Kebersihan & Pembangunan'),
                      ),
                    ],
                    onChanged: (value) {
                      if (value != null) {
                        setState(() => _jabatan = value);
                      }
                    },
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Nama
              AppTextField(
                controller: _fullNameCtrl,
                label: 'Nama',
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

              // Telepon
              AppTextField(
                controller: _phoneCtrl,
                label: 'Telepon',
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
              const SizedBox(height: AppSpacing.lg),

              // Horizontal buttons: Simpan + Batal
              Row(
                children: [
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.accent.withValues(alpha: 0.15),
                            blurRadius: 8,
                            offset: const Offset(0, 0),
                          ),
                        ],
                      ),
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _save,
                          style: OutlinedButton.styleFrom(
                            side: const BorderSide(color: AppColors.accent),
                            foregroundColor: AppColors.accent,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),
                          child: _saving
                              ? SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                    backgroundColor: Colors.transparent,
                                    valueColor: AlwaysStoppedAnimation<Color>(
                                      AppColors.accent,
                                    ),
                                  ),
                                )
                              : const Row(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    Icon(Icons.save_outlined, size: 18),
                                    SizedBox(width: 6),
                                    Flexible(
                                      child: FittedBox(
                                        fit: BoxFit.scaleDown,
                                        child: Text('Simpan Perubahan'),
                                      ),
                                    ),
                                  ],
                                ),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.danger.withValues(alpha: 0.15),
                            blurRadius: 8,
                            offset: const Offset(0, 0),
                          ),
                        ],
                      ),
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _confirmCancel,
                          style: OutlinedButton.styleFrom(
                            side: BorderSide(color: AppColors.danger),
                            foregroundColor: AppColors.danger,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),
                          child: const Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.close, size: 18),
                              SizedBox(width: 6),
                              Flexible(
                                child: FittedBox(
                                  fit: BoxFit.scaleDown,
                                  child: Text('Batal'),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.base),
            ],
          ),
        ),
      ),
    );
  }
}
