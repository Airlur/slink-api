-- 短链分享信息表（shares）
CREATE TABLE IF NOT EXISTS `shares` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '分享信息唯一标识',
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `share_title` VARCHAR(50) NULL DEFAULT NULL COMMENT '分享标题',
  `share_desc` VARCHAR(100) NULL DEFAULT NULL COMMENT '分享描述',
  `share_image` VARCHAR(255) NULL DEFAULT NULL COMMENT '分享封面图URL',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` DATETIME NULL DEFAULT NULL COMMENT '软删除标记（非NULL表示已删除）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_short_code` (`short_code`),
  INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短链接分享信息表';